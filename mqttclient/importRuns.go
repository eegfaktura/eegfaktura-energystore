package mqttclient

import "at.ourproject/energystore/model"

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
