package store

import (
	"fmt"
	"sort"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/golang/glog"
)

// DayBlock is the energy of one metering point for one day, as mqttclient.SplitEnergyByDay cuts a CR_MSG.
type DayBlock struct {
	MeteringPoint string
	Energy        *model.MqttEnergy
	// Counts, if set, are the consumer/producer counts StoreEnergyV2 would see for this block (from
	// RegisterMeteringPoints over the messages in their order); nil: taken from the order of the batch.
	Counts *model.CounterPointMetaInfo
}

// StoreDayBatch stores the day blocks of one store and one day with one read and one write of the rows and
// of cpmeta/0, instead of one each per block. The result is the same as StoreEnergyV2 for every block in the
// given order: the same SourceIdx for new metering points, the same rows and the same periods in the meta.
// Errors are wrapped like in StoreEnergyV2 (ErrMetaRecord, "read stored rows").
func StoreDayBatch(db *ebow.BowStorage, blocks []DayBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	meta, counts, err := registerBlocks(db, blocks)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMetaRecord, err)
	}

	begin, end := time.UnixMilli(blocks[0].Energy.Start), time.UnixMilli(blocks[0].Energy.End)
	for _, b := range blocks[1:] {
		if s := time.UnixMilli(b.Energy.Start); s.Before(begin) {
			begin = s
		}
		if e := time.UnixMilli(b.Energy.End); e.After(end) {
			end = e
		}
	}

	resources := &map[string]*model.RawSourceLine{}
	monitorStart := time.Now()
	// A read error must abort the store (#31), see StoreEnergyV2.
	if err := fetchSourceRange(db, "CP", begin.Local(), end.Local(), resources); err != nil {
		return fmt.Errorf("read stored rows: %w", err)
	}
	glog.V(4).Infof("Fetching source for %d blocks takes %v", len(blocks), time.Since(monitorStart).Milliseconds())

	fetched := make(map[string]bool, len(*resources))
	for id := range *resources {
		fetched[id] = true
	}
	var created []*model.RawSourceLine // rows that earlier blocks of the batch created
	for i, b := range blocks {
		metaMeter := organizeMetaCodeImport(b.Energy.Data)
		if len(metaMeter) == 0 {
			continue
		}
		evenCreatedRows(created, b.Energy)
		n := len(*resources)
		// Rows a block creates are sized by the counts StoreEnergyV2 would see at this block.
		c := counts[i]
		if b.Counts != nil {
			c = *b.Counts
		}
		resources, err = importEnergyValuesV2(db.GetTenant(), metaMeter, b.Energy, meta[b.MeteringPoint],
			c.ConsumerCount, c.ProducerCount, resources, false)
		if err != nil {
			return err
		}
		if len(*resources) > n {
			created = created[:0]
			for id, l := range *resources {
				if !fetched[id] {
					created = append(created, l)
				}
			}
		}
	}

	updated := make([]*model.RawSourceLine, 0, len(*resources))
	for _, v := range *resources {
		updated = append(updated, v)
	}
	sort.Slice(updated, func(i, j int) bool {
		return updated[i].Id < updated[j].Id
	})

	monitorStart = time.Now()
	if err := db.SetLines(updated); err != nil {
		glog.Error(err)
		return err
	}
	glog.V(4).Infof("Writing source for %d blocks takes %v", len(blocks), time.Since(monitorStart).Milliseconds())

	if err := widenMetaPeriods(db, blocks); err != nil {
		return fmt.Errorf("%w: %w", ErrMetaRecord, err)
	}
	return nil
}

// evenCreatedRows makes the QoV arrays of rows created by earlier blocks as long as their value arrays when
// the block's day range covers them. StoreEnergyV2 reads such a row back for the next block, and the copy
// on reading (RawSourceLine.Copy) does the same; otherwise the stored arrays would differ in length.
func evenCreatedRows(rows []*model.RawSourceLine, e *model.MqttEnergy) {
	from, until := dayKey(time.UnixMilli(e.Start).Local()), dayKey(time.UnixMilli(e.End).Local())
	for _, l := range rows {
		if day := l.Id[len("CP/"):len("CP/yyyy/mm/dd/")]; day < from || day > until {
			continue
		}
		l.QoVConsumers = resizeInts(l.QoVConsumers, len(l.Consumers))
		l.QoVProducers = resizeInts(l.QoVProducers, len(l.Producers))
	}
}

// dayKey is the day part of a row id, as fetchSourceRange builds its range.
func dayKey(t time.Time) string {
	return fmt.Sprintf("%.4d/%.2d/%.2d/", t.Year(), int(t.Month()), t.Day())
}

func resizeInts(s []int, n int) []int {
	if len(s) == n {
		return s
	}
	r := make([]int, n)
	copy(r, s)
	return r
}

// RegisterMeteringPoints registers the metering points of the blocks that cpmeta/0 does not know yet, in the
// order of the blocks and with one write, and returns per block the counts after its metering point. With
// the first block of every message in message order this assigns ID and SourceIdx as single imports of the
// messages in that order would. Errors are wrapped as ErrMetaRecord.
func RegisterMeteringPoints(db *ebow.BowStorage, blocks []DayBlock) ([]model.CounterPointMetaInfo, error) {
	_, counts, err := registerBlocks(db, blocks)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMetaRecord, err)
	}
	return counts, nil
}

// registerBlocks reads cpmeta/0 once and registers the metering points it does not know yet in one write,
// in the order of the blocks, as PrepareMetaInfoMap would block by block. It returns the meta and, per
// block, the counts after its metering point was registered.
func registerBlocks(db *ebow.BowStorage, blocks []DayBlock) (map[string]*model.CounterPointMeta, []model.CounterPointMetaInfo, error) {
	info, meta, err := CalcMetaInfo(db)
	if err != nil {
		return nil, nil, err
	}
	modified := false
	counts := make([]model.CounterPointMetaInfo, len(blocks))
	for i, b := range blocks {
		if registerMeteringPoint(meta, info, b.MeteringPoint, utils.ExamineDirection(b.Energy.Data)) {
			modified = true
		}
		counts[i] = *info
	}
	if modified {
		if err := db.SetMeta(rawMetaOf(meta)); err != nil {
			return nil, nil, err
		}
	}
	return meta, counts, nil
}

// widenMetaPeriods widens the periods of the blocks' metering points in cpmeta/0 with one read and at most
// one write, by the rule of updateMetaCP, applied against the record on disk (see updateMeta).
func widenMetaPeriods(db ebow.IBowStorage, blocks []DayBlock) error {
	meta, err := db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))
	if err != nil {
		return err
	}
	byName := make(map[string]*model.CounterPointMeta, len(meta.CounterPoints))
	for _, m := range meta.CounterPoints {
		if _, ok := byName[m.Name]; !ok { // updateMeta takes the first match
			byName[m.Name] = m
		}
	}
	changed := false
	for _, b := range blocks {
		if m := byName[b.MeteringPoint]; m != nil && updateMetaCP(m, time.UnixMilli(b.Energy.Start), time.UnixMilli(b.Energy.End)) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return db.SetMeta(meta)
}
