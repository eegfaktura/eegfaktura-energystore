package mqttclient

import (
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

// ExecuteBatch decodes, stores and confirms a batch of CR_MSG of this tenant. The day blocks of all messages
// of an ecId are stored together, one store.StoreDayBatch per day. Outcome, log line and cr_msg_history reply
// are per message as in Execute; the caller acks every message after ExecuteBatch returns (#19).
func (tmw *TenantEnergyImporter) ExecuteBatch(msgs []mqtt.Message) {
	monitor := time.Now()
	data := make([]*model.MqttEnergyMessage, 0, len(msgs))
	for _, msg := range msgs {
		d := decodeMessage(msg.Payload())
		if d == nil {
			glog.Errorf("Data without content. tenant=%s", tmw.Tenant)
			continue
		}
		data = append(data, d)
	}

	errs := tmw.ImportBatch(data)
	for i, d := range data {
		if errs[i] != nil {
			glog.Errorf("energy data not stored, no cr_msg_history reply: %v tenant=%s ecId=%s meteringPoint=%s conversationId=%s",
				errs[i], tmw.Tenant, d.EcId, d.Meter.MeteringPoint, d.ConversationId)
			continue
		}
		tmw.streamer.SendMessage(tmw.Tenant, d)
	}
	glog.V(3).Infof("Execution of %d messages finished in %d ms (%v)", len(msgs), time.Since(monitor).Milliseconds(), tmw.Tenant)
}

// ImportBatch stores the messages and returns the error of each one (nil: stored), with the same errors as
// Import. A message counts as stored when all its day blocks are stored.
//   - A day that fails as a batch (a read, write or corrupt row) is stored again block by block, so the
//     error only reaches the messages that have it.
//   - A meta record error (store.ErrMetaRecord) hands the unfinished messages of that ecId to Import, which
//     reports it per message as before.
func (tmw *TenantEnergyImporter) ImportBatch(msgs []*model.MqttEnergyMessage) []error {
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
		tmw.importEcId(ecId, msgs, byEcId[ecId], errs)
	}
	return errs
}

func (tmw *TenantEnergyImporter) importEcId(ecId string, msgs []*model.MqttEnergyMessage, idx []int, errs []error) {
	st, err := tmw.ensureDb(ecId)
	if err != nil {
		for _, i := range idx {
			errs[i] = fmt.Errorf("open store ecId=%s: %w", ecId, err)
		}
		return
	}
	runs := splitRuns(msgs, idx)
	for r, run := range runs {
		if tmw.importRun(st, msgs, run, errs) {
			continue
		}
		for _, rest := range runs[r+1:] {
			for _, i := range rest {
				errs[i] = tmw.Import(msgs[i])
			}
		}
		return
	}
}

// splitRuns cuts the messages of one ecId into runs in which no metering point has two messages with
// overlapping periods. A run is stored day by day; a later message for the same quarter hours must still
// overwrite an earlier one, also where SplitEnergyByDay cut their days differently (after a DST change the
// days of a longer message start at 23:00).
func splitRuns(msgs []*model.MqttEnergyMessage, idx []int) [][]int {
	var runs [][]int
	var run []int
	periods := map[string][][2]int64{}
	for _, i := range idx {
		m := msgs[i]
		mp := m.Meter.MeteringPoint
		if overlapsAny(m.Energy, periods[mp]) {
			runs = append(runs, run)
			run, periods = nil, map[string][][2]int64{}
		}
		run = append(run, i)
		for _, e := range m.Energy {
			periods[mp] = append(periods[mp], [2]int64{e.Start, e.End})
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}
	return runs
}

func overlapsAny(energy []model.MqttEnergy, periods [][2]int64) bool {
	for _, e := range energy {
		for _, p := range periods {
			if e.Start <= p[1] && p[0] <= e.End {
				return true
			}
		}
	}
	return false
}

// importRun stores the messages of one run, day by day in ascending order. It returns false after a meta
// record error; the unfinished messages of the run have then been imported one by one.
func (tmw *TenantEnergyImporter) importRun(st *ebow.BowStorage, msgs []*model.MqttEnergyMessage, run []int, errs []error) bool {
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

	for _, k := range keys {
		if !metaOk {
			break
		}
		if metaOk = tmw.storeDay(st, days[k].blocks, days[k].owners, pending, msgErrs); !metaOk {
			break
		}
	}
	for _, i := range run {
		if !metaOk && pending[i] > 0 {
			errs[i] = tmw.Import(msgs[i])
			continue
		}
		errs[i] = msgErrs[i].err()
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
