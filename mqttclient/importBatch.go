package mqttclient

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
)

// errStopped: the import of the message was stopped because the dispatcher closes; it stays unacked and
// mosquitto delivers it again (the import is idempotent).
var errStopped = errors.New("import stopped, energystore shuts down")

// ExecuteBatch decodes, stores and confirms a batch of CR_MSG of this tenant. The day blocks of all messages
// of an ecId are stored together, one store.StoreDayBatch per day. Outcome, log line and cr_msg_history reply
// are per message as in Execute. It returns which messages were processed; the caller acks those (#19).
// When ctx ends it stops between messages, runs and days; the messages not finished by then are not.
func (tmw *TenantEnergyImporter) ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool {
	monitor := time.Now()
	finished := make([]bool, len(msgs))
	data := make([]*model.MqttEnergyMessage, 0, len(msgs))
	origin := make([]int, 0, len(msgs))
	for i, msg := range msgs {
		d := decodeMessage(msg.Payload())
		if d == nil {
			glog.Errorf("Data without content. tenant=%s", tmw.Tenant)
			finished[i] = true
			continue
		}
		data = append(data, d)
		origin = append(origin, i)
	}

	errs := tmw.ImportBatch(ctx, data)
	for i, d := range data {
		if errs[i] == errStopped {
			glog.Infof("%v, message not acked: tenant=%s ecId=%s meteringPoint=%s conversationId=%s",
				errs[i], tmw.Tenant, d.EcId, d.Meter.MeteringPoint, d.ConversationId)
			continue
		}
		finished[origin[i]] = true
		if errs[i] != nil {
			glog.Errorf("energy data not stored, no cr_msg_history reply: %v tenant=%s ecId=%s meteringPoint=%s conversationId=%s",
				errs[i], tmw.Tenant, d.EcId, d.Meter.MeteringPoint, d.ConversationId)
			continue
		}
		tmw.streamer.SendMessage(tmw.Tenant, d)
	}
	glog.V(3).Infof("Execution of %d messages finished in %d ms (%v)", len(msgs), time.Since(monitor).Milliseconds(), tmw.Tenant)
	return finished
}

// ImportBatch stores the messages and returns the error of each one (nil: stored), with the same errors as
// Import. A message counts as stored when all its day blocks are stored.
//   - A day that fails as a batch (a read, write or corrupt row) is stored again block by block, so the
//     error only reaches the messages that have it.
//   - A meta record error (store.ErrMetaRecord) hands the unfinished messages of that ecId to Import, which
//     reports it per message as before.
//   - When ctx ends, the messages not finished by then get errStopped.
func (tmw *TenantEnergyImporter) ImportBatch(ctx context.Context, msgs []*model.MqttEnergyMessage) []error {
	errs := make([]error, len(msgs))
	var ecIds []string
	byEcId := map[string][]int{}
	for i, m := range msgs {
		if _, ok := byEcId[m.EcId]; !ok {
			ecIds = append(ecIds, m.EcId)
		}
		byEcId[m.EcId] = append(byEcId[m.EcId], i)
	}
	for _, ecId := range ecIds {
		tmw.importEcId(ctx, ecId, msgs, byEcId[ecId], errs)
	}
	return errs
}

// importOne is Import, or errStopped once ctx has ended.
func (tmw *TenantEnergyImporter) importOne(ctx context.Context, msg *model.MqttEnergyMessage) error {
	if ctx.Err() != nil {
		return errStopped
	}
	return tmw.Import(msg)
}

func (tmw *TenantEnergyImporter) importEcId(ctx context.Context, ecId string, msgs []*model.MqttEnergyMessage, idx []int, errs []error) {
	if ctx.Err() != nil {
		for _, i := range idx {
			errs[i] = errStopped
		}
		return
	}
	st, err := tmw.ensureDb(ecId)
	if err != nil {
		for _, i := range idx {
			errs[i] = fmt.Errorf("open store ecId=%s: %w", ecId, err)
		}
		return
	}
	runs := splitRuns(msgs, idx)
	for r, run := range runs {
		switch {
		case ctx.Err() != nil:
			for _, i := range run.idx {
				errs[i] = errStopped
			}
		case run.single:
			errs[run.idx[0]] = tmw.Import(msgs[run.idx[0]])
		case !tmw.importRun(ctx, st, msgs, run.idx, errs):
			for _, rest := range runs[r+1:] {
				for _, i := range rest.idx {
					errs[i] = tmw.importOne(ctx, msgs[i])
				}
			}
			return
		}
	}
}

// run is a sequence of messages of one ecId stored together. single: one message whose own energy entries
// overlap; it is stored with Import, entry by entry, so its later entry wins as before.
type run struct {
	idx    []int
	single bool
}

// splitRuns cuts the messages of one ecId into runs in which no metering point has two messages with
// overlapping periods (Start inclusive, End exclusive as in a CR_MSG: back-to-back days share a run). A run is stored day by day; a later message for the same quarter hours must still
// overwrite an earlier one, also where SplitEnergyByDay cut their days differently (after a DST change the
// days of a longer message start at 23:00).
func splitRuns(msgs []*model.MqttEnergyMessage, idx []int) []run {
	var runs []run
	var cur run
	periods := map[string][][2]int64{}
	flush := func() {
		if len(cur.idx) > 0 {
			runs = append(runs, cur)
		}
		cur, periods = run{}, map[string][][2]int64{}
	}
	for _, i := range idx {
		m := msgs[i]
		mp := m.Meter.MeteringPoint
		if selfOverlaps(m.Energy) {
			flush()
			runs = append(runs, run{idx: []int{i}, single: true})
			continue
		}
		if overlapsAny(m.Energy, periods[mp]) {
			flush()
		}
		cur.idx = append(cur.idx, i)
		for _, e := range m.Energy {
			periods[mp] = append(periods[mp], [2]int64{e.Start, e.End})
		}
	}
	flush()
	return runs
}

func selfOverlaps(energy []model.MqttEnergy) bool {
	for i := range energy {
		for _, e := range energy[i+1:] {
			if e.Start < energy[i].End && energy[i].Start < e.End {
				return true
			}
		}
	}
	return false
}

func overlapsAny(energy []model.MqttEnergy, periods [][2]int64) bool {
	for _, e := range energy {
		for _, p := range periods {
			if e.Start < p[1] && p[0] < e.End {
				return true
			}
		}
	}
	return false
}

// importRun stores the messages of one run, day by day in ascending order. It returns false after a meta
// record error; the unfinished messages of the run have then been imported one by one. When ctx ends it
// stops before the next day; the unfinished messages get errStopped.
func (tmw *TenantEnergyImporter) importRun(ctx context.Context, st *ebow.BowStorage, msgs []*model.MqttEnergyMessage, run []int, errs []error) bool {
	type day struct {
		blocks []store.DayBlock
		owners []int // message index per block
	}
	msgBlocks := make([][]store.DayBlock, len(run))
	var firsts []store.DayBlock
	msgErrs := map[int]*importErrors{}
	for r, i := range run {
		msgErrs[i] = &importErrors{}
		for e := range msgs[i].Energy {
			blocks := SplitEnergyByDay(msgs[i].Energy[e])
			for n := range blocks {
				msgBlocks[r] = append(msgBlocks[r], store.DayBlock{MeteringPoint: msgs[i].Meter.MeteringPoint, Energy: &blocks[n]})
			}
		}
		if len(msgBlocks[r]) > 0 {
			firsts = append(firsts, msgBlocks[r][0])
		}
	}
	// New metering points get ID and SourceIdx in message order, as Import message by message would give
	// them, and every block sizes new rows with the counts of its message (not in day order).
	counts, err := store.RegisterMeteringPoints(st, firsts)
	metaOk := err == nil
	if !metaOk {
		glog.Errorf("Error storing Energy: %v (%d messages) tenant=%s", err, len(run), tmw.Tenant)
	}

	days := map[int64]*day{}
	pending := map[int]int{} // blocks per message not yet stored or failed
	f := 0
	for r, i := range run {
		var c *model.CounterPointMetaInfo
		if metaOk && len(msgBlocks[r]) > 0 {
			c = &counts[f]
			f++
		}
		for _, b := range msgBlocks[r] {
			b.Counts = c
			key := dayStart(b.Energy.Start)
			if days[key] == nil {
				days[key] = &day{}
			}
			days[key].blocks = append(days[key].blocks, b)
			days[key].owners = append(days[key].owners, i)
			pending[i]++
		}
	}
	keys := make([]int64, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })

	stopped := false
	for _, k := range keys {
		if stopped = ctx.Err() != nil; stopped || !metaOk {
			break
		}
		if metaOk = tmw.storeDay(st, days[k].blocks, days[k].owners, pending, msgErrs); !metaOk {
			break
		}
	}
	for _, i := range run {
		switch {
		case pending[i] > 0 && stopped:
			errs[i] = errStopped
		case pending[i] > 0 && !metaOk:
			errs[i] = tmw.importOne(ctx, msgs[i])
		default:
			errs[i] = msgErrs[i].err()
		}
	}
	return metaOk
}

// storeDay stores the blocks of one day as a batch, or block by block if the batch fails for another reason
// than the meta record. It returns false at a meta record error, leaving the blocks not done as pending.
func (tmw *TenantEnergyImporter) storeDay(st *ebow.BowStorage, blocks []store.DayBlock, owners []int, pending map[int]int, msgErrs map[int]*importErrors) bool {
	err := store.StoreDayBatch(st, blocks)
	if err == nil {
		for _, i := range owners {
			pending[i]--
		}
		return true
	}
	if errors.Is(err, store.ErrMetaRecord) {
		glog.Errorf("Error storing Energy: %v (%d day blocks) tenant=%s", err, len(blocks), tmw.Tenant)
		return false
	}
	glog.Warningf("day batch of %d blocks not stored, storing them one by one: %v tenant=%s", len(blocks), err, tmw.Tenant)
	for j, b := range blocks {
		if err := store.StoreEnergyV2(st, b.MeteringPoint, b.Energy); err != nil {
			glog.Errorf("Error storing Energy: %v (Metering-Point: %s)", err, b.MeteringPoint)
			if errors.Is(err, store.ErrMetaRecord) {
				return false
			}
			msgErrs[owners[j]].add(blockError(b.Energy, err))
		}
		pending[owners[j]]--
	}
	return true
}
