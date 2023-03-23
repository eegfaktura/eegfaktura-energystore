package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/golang/glog"
	"github.com/xuri/excelize/v2"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var dateLine = regexp.MustCompile(`^[0-9]{2}.[0-9]{2}.[0-9]{4}\s[0-9]{2}:[0-9]{2}:[0-9]{2}$`)

func OpenExceFile(path string) (*excelize.File, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return f, nil
}

func OpenReader(r io.Reader, filename string, opt ...excelize.Options) (*excelize.File, error) {
	f, err := excelize.OpenReader(r, opt...)
	if err != nil {
		return nil, err
	}
	f.Path = filename
	return f, nil
}

type MeterCodeType int

const (
	Total MeterCodeType = iota
	Share
	Coverage
	Profit
	Bad
)

type excelHeader struct {
	meteringPointId map[int]string
	energyDirection map[int]string
	periodStart     map[int]string
	periodEnd       map[int]string
	meterCode       map[int]MeterCodeType
}

type excelCounterPointMeta struct {
	*model.CounterPointMeta
	Idx   int
	IdxG2 int
	IdxG3 int
}

func ImportExcelEnergyFile(f *excelize.File, sheet string, db *store.BowStorage) ([]int, error) {
	rows, err := f.Rows(sheet)
	if err != nil {
		fmt.Println(err)
		return []int{}, err
	}
	fmt.Printf("Rows: %+v\n", rows.Error())

	var rIdx int = 1
	var rawDatas []*model.RawSourceLine = []*model.RawSourceLine{}
	var rawDatasG2 []*model.RawSourceLine = []*model.RawSourceLine{}
	var rawDatasG3 []*model.RawSourceLine = []*model.RawSourceLine{}

	var excelHeader excelHeader
	excelHeaderInitialized := false

	var excelCpMeta map[int]*excelCounterPointMeta
	var updatedCpMeta []*model.CounterPointMeta
	var yearSet map[int]bool = make(map[int]bool)

	t := time.Now()
	for rows.Next() {
		if cols, err := rows.Columns(excelize.Options{RawCellValue: false}); err == nil && len(cols) > 0 {
			switch cols[0] {
			case "MeteringpointID":
				excelHeader.meteringPointId = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.meteringPointId[i] = c
				}
			case "Energy direction":
				excelHeader.energyDirection = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.energyDirection[i] = c
				}
			case "Period end":
				excelHeader.periodEnd = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.periodEnd[i] = c
				}
			case "Period start":
				excelHeader.periodStart = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.periodStart[i] = c
				}
			case "Metercode":
				excelHeader.meterCode = make(map[int]MeterCodeType, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.meterCode[i] = returnMeterCode(strings.ToUpper(c))
				}
			default:
				switch {
				case dateLine.MatchString(cols[0]):
					if !excelHeaderInitialized {
						excelCpMeta, updatedCpMeta, err = buildMatrixMetaStruct(db, excelHeader)
						excelHeaderInitialized = true
					}
					var y, m, d, hh, mm, ss int
					rawData := &model.RawSourceLine{Consumers: []float64{}, Producers: []float64{}}
					if _, err := fmt.Sscanf(cols[0], "%d.%d.%d %d:%d:%d", &d, &m, &y, &hh, &mm, &ss); err == nil {
						rawData.Id = fmt.Sprintf("CP-G.01/%d/%.2d/%.2d/%.2d/%.2d/%.2d", y, m, d, hh, mm, ss)
						yearSet[y] = true
					} else {
						glog.Infof("Error Time parsing: %s (%s)", err, cols[0])
						continue
					}

					//
					// Insert G1 values
					//
					if len(rawDatas) == 4900 {
						te := 1
						println(te)
					}
					rawData.Id = fmt.Sprintf("CP-G.01/%d/%.2d/%.2d/%.2d/%.2d/%.2d", y, m, d, hh, mm, ss)
					_ = db.GetLine(rawData)
					for i := 0; i < len(excelCpMeta); i++ {
						v := excelCpMeta[i]
						value := returnFloat(cols[v.Idx+1])
						switch v.Dir {
						case "CONSUMPTION":
							rawData.Consumers = utils.Insert(rawData.Consumers, v.SourceIdx, value)
							v.Count += 1
						case "GENERATION":
							rawData.Producers = utils.Insert(rawData.Producers, v.SourceIdx, value)
							v.Count += 1
						}
					}
					rawDatas = append(rawDatas, rawData)

					//
					// Insert G2 values
					//
					rawDataG2 := &model.RawSourceLine{Consumers: []float64{}, Producers: []float64{}}
					rawDataG2.Id = fmt.Sprintf("CP-G.02/%d/%.2d/%.2d/%.2d/%.2d/%.2d", y, m, d, hh, mm, ss)
					_ = db.GetLineG2(rawDataG2)
					for i := 0; i < len(excelCpMeta); i++ {
						v := excelCpMeta[i]
						if v.IdxG2 < 0 {
							continue
						}
						value := returnFloat(cols[v.IdxG2+1])
						switch v.Dir {
						case "CONSUMPTION":
							rawDataG2.Consumers = utils.Insert(rawDataG2.Consumers, v.SourceIdx, value)
							v.Count += 1
						case "GENERATION":
							rawDataG2.Producers = utils.Insert(rawDataG2.Producers, v.SourceIdx, value)
							v.Count += 1
						}
					}
					rawDatasG2 = append(rawDatasG2, rawDataG2)

					//
					// Insert G3 values
					//
					rawDataG3 := &model.RawSourceLine{Consumers: []float64{}, Producers: []float64{}}
					rawDataG3.Id = fmt.Sprintf("CP-G.03/%d/%.2d/%.2d/%.2d/%.2d/%.2d", y, m, d, hh, mm, ss)
					_ = db.GetLineG3(rawData)
					for i := 0; i < len(excelCpMeta); i++ {
						v := excelCpMeta[i]
						if v.IdxG3 < 0 {
							continue
						}
						value := returnFloat(cols[v.IdxG3+1])
						switch v.Dir {
						case "CONSUMPTION":
							rawDataG3.Consumers = utils.Insert(rawDataG3.Consumers, v.SourceIdx, value)
							v.Count += 1
						case "GENERATION":
							rawDataG3.Producers = utils.Insert(rawDataG3.Producers, v.SourceIdx, value)
							v.Count += 1
						}
					}
					rawDatasG3 = append(rawDatasG3, rawDataG3)
					//
					//
					//
					rIdx += 1
				}
			}
		}
	}
	fmt.Printf("Time taken via read file: %v\n", time.Since(t))
	if err := db.SetLines(rawDatas); err != nil {
		return []int{}, err
	}
	if err := db.SetLinesG2(rawDatasG2); err != nil {
		return []int{}, err
	}
	if err := db.SetLinesG3(rawDatasG3); err != nil {
		return []int{}, err
	}

	rawMeta := &model.RawSourceMeta{Id: fmt.Sprintf("cpmeta/%d", 0), CounterPoints: updatedCpMeta, NumberOfMetering: rIdx}
	err = db.SetMeta(rawMeta)

	if err != nil {
		glog.Error(err.Error())
		return []int{}, err
	}
	fmt.Printf("Time taken via write batch: %v\n", time.Since(t))

	years := []int{}
	for k, _ := range yearSet {
		years = append(years, k)
	}
	return years, nil
}

func buildMatrixMetaStruct(db *store.BowStorage, excelHeader excelHeader) (map[int]*excelCounterPointMeta, []*model.CounterPointMeta, error) {
	type pair struct {
		key   string
		value int
		vG2   int
		vG3   int
	}
	msSet := map[string]pair{}
	meteringIdSet := map[string]int{}
	for i := 0; i < len(excelHeader.meteringPointId); i++ {
		if i < len(excelHeader.meterCode) {
			v := excelHeader.meteringPointId[i]
			if v == "AT0030000000000000000000030032764" {
				pe := 1
				println(pe)
			}
			if excelHeader.meterCode[i] == Total {
				if _, ok := meteringIdSet[v]; !ok && strings.ToLower(v) != "total" {
					meteringIdSet[v] = i
					if _ms, ok := msSet[v]; ok {
						_ms.value = i
						msSet[v] = _ms
					} else {
						msSet[v] = pair{v, i, -1, -1}
					}
				}
			} else if strings.ToLower(v) != "total" && (excelHeader.meterCode[i] == Share || excelHeader.meterCode[i] == Profit) {
				if _ms, ok := msSet[v]; ok {
					_ms.vG2 = i
					msSet[v] = _ms
				} else {
					msSet[v] = pair{v, -1, i, -1}
				}
			} else if strings.ToLower(v) != "total" && excelHeader.meterCode[i] == Coverage {
				if _ms, ok := msSet[v]; ok {
					_ms.vG3 = i
					msSet[v] = _ms
				} else {
					msSet[v] = pair{v, -1, -1, i}
				}
			}
		}
	}

	ms := []pair{}
	for _, v := range msSet {
		if !(v.value < 0) {
			ms = append(ms, v)
		}
	}

	sort.Slice(ms, func(i, j int) bool {
		return ms[i].value < ms[j].value
	})

	excelCpMeta := make(map[int]*excelCounterPointMeta, len(ms))
	storedCpMeta, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return nil, nil, err
	}
	for i, kv := range ms {
		_, ok := storedCpMeta[kv.key]
		if !ok {
			meterpoint := kv.key
			switch excelHeader.energyDirection[kv.value] {
			case model.CONSUMER_DIRECTION:
				metaInfo.ConsumerCount += 1
				metaInfo.MaxConsumerIdx += 1
				storedCpMeta[meterpoint] = &model.CounterPointMeta{
					ID:          fmt.Sprintf("%.3d", len(storedCpMeta)),
					SourceIdx:   metaInfo.MaxConsumerIdx,
					Name:        meterpoint,
					Dir:         model.CONSUMER_DIRECTION,
					PeriodStart: excelHeader.periodStart[kv.value],
				}
			case model.PRODUCER_DIRECTION:
				metaInfo.ProducerCount += 1
				metaInfo.MaxProducerIdx += 1
				storedCpMeta[meterpoint] = &model.CounterPointMeta{
					ID:          fmt.Sprintf("%.3d", len(storedCpMeta)),
					SourceIdx:   metaInfo.MaxProducerIdx,
					Name:        meterpoint,
					Dir:         model.PRODUCER_DIRECTION,
					PeriodStart: excelHeader.periodStart[kv.value],
				}
			}
		}
		storedMeta := storedCpMeta[kv.key]
		nStoredPeriodEnd, _ := utils.ParseTime(storedMeta.PeriodEnd)
		nExcelPeriodEnd, _ := utils.ParseTime(excelHeader.periodEnd[kv.value])
		if nExcelPeriodEnd.Unix() > nStoredPeriodEnd.Unix() {
			storedMeta.PeriodEnd = excelHeader.periodEnd[kv.value]
		}

		nStoredPeriodStart, _ := utils.ParseTime(storedMeta.PeriodStart)
		nExcelPeriodStart, _ := utils.ParseTime(excelHeader.periodStart[kv.value])
		if nStoredPeriodStart.Unix() > nExcelPeriodStart.Unix() {
			storedMeta.PeriodStart = excelHeader.periodStart[kv.value]
		}

		excelCpMeta[i] = &excelCounterPointMeta{CounterPointMeta: storedMeta}
		switch excelHeader.energyDirection[kv.value] {
		case model.PRODUCER_DIRECTION:
			excelCpMeta[i].Idx = kv.value
			excelCpMeta[i].IdxG2 = kv.vG2
			excelCpMeta[i].IdxG3 = kv.vG3
		default:
			excelCpMeta[i].Idx = kv.value
			excelCpMeta[i].IdxG2 = kv.vG2
			excelCpMeta[i].IdxG3 = kv.vG3
		}
	}

	updateCpMeta := []*model.CounterPointMeta{}
	for _, v := range storedCpMeta {
		updateCpMeta = append(updateCpMeta, v)
	}

	sort.Slice(updateCpMeta, func(i, j int) bool {
		return updateCpMeta[i].SourceIdx < updateCpMeta[j].SourceIdx
	})
	fmt.Println("ExcelMeta:")
	for k, v := range excelCpMeta {
		fmt.Printf("Key: %+v Value: %+v\n", k, v)
	}
	fmt.Println("UpdateMeta:")
	for i, v := range updateCpMeta {
		fmt.Printf("Idx: %+v Value: %+v\n", i, v)
	}
	return excelCpMeta, updateCpMeta, nil
}

func returnInt(c string) int {
	if len(c) == 0 {
		return 0
	}
	i, err := strconv.Atoi(c)
	if err != nil {
		return 0
	}
	return i
}

func returnFloat(c string) float64 {
	if len(c) == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(c, 64)
	if err != nil {
		return 0
	}
	return f
}

func returnMeterCode(c string) MeterCodeType {
	switch {
	case strings.Contains(c, "GESAMTVERBRAUCH"):
		return Total
	case strings.Contains(c, "GESAMTE"):
		return Total
	case strings.Contains(c, "ANTEIL"):
		return Share
	case strings.Contains(c, "EIGENDECKUNG"):
		return Coverage
	case strings.Contains(c, "ÜBERSCHUSSERZEUGUNG"):
		return Profit
	default:
		return Bad
	}
}

func convertExcelMeterCode(code MeterCodeType) string {
	switch code {
	case Total:
		return "G.01"
	case Share:
		return "G.02"
	case Coverage:
		return "G.03"
	case Profit:
		return "G.02"
	}
	return ""
}
