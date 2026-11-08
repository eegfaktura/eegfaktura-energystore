package excel

import (
	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var dateLine = regexp.MustCompile(`^[0-9]{2}\.[0-9]{2}\.[0-9]{4}\s[0-9]{2}:[0-9]{2}(?::[0-9]{2})?$`)
var numberPattern = regexp.MustCompile(`^[0-9\\.,]+$`)

// openOptions are the unzip limits for every workbook this package opens.
// excelize defaults to an unzip limit of about 16 GB, so a small crafted
// workbook (zip bomb) could fill memory and the temp directory.
//
// UnzipSizeLimit caps the running total of the uncompressed sizes declared by
// the zip entries; excelize stops at the first entry that crosses it (entries
// read before that are already unzipped). Uploads reach the service through
// gqlgen's multipart transport, whose default MaxUploadSize is 32 MiB (server.go:
// handler.NewDefaultServer). The workbooks in test/ expand 7x to 27x (largest
// ratio: test/ShortTest-Energiedaten New.xlsx, 0.13 MB -> 3.4 MB, 26.7x; the
// largest file, test/zaehlpunkte-beispieldatei.xlsx, 21.7 MB -> 153 MB, 7.1x).
// 1 GiB is the 32 MiB cap times 32, above the largest observed ratio.
//
// UnzipXMLSizeLimit is the excelize default (16 MiB): worksheets and the shared
// string table above it are unzipped to the temp directory instead of memory.
// It is set explicitly because it must not exceed UnzipSizeLimit.
var openOptions = excelize.Options{
	UnzipSizeLimit:    1 << 30,
	UnzipXMLSizeLimit: 16 << 20,
}

func OpenExceFile(path string) (*excelize.File, error) {
	f, err := excelize.OpenFile(path, openOptions)
	if err != nil {
		if f != nil {
			_ = f.Close()
		}
		glog.Error(err)
		return nil, err
	}
	return f, nil
}

// OpenReader opens an uploaded workbook. Its temp files go to a directory of
// their own: when opening fails while unzipping, excelize returns no File and
// so cannot remove the temp files written so far; the directory is removed
// instead (also if excelize panics). The returned func closes the File and
// removes the directory; call it when done with the File.
func OpenReader(r io.Reader, filename string) (*excelize.File, func(), error) {
	tmpDir, err := os.MkdirTemp("", "energystore-import-")
	if err != nil {
		return nil, nil, err
	}
	keepTmpDir := false
	defer func() {
		if !keepTmpDir {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	opts := openOptions
	opts.TmpDir = tmpDir
	f, err := excelize.OpenReader(r, opts)
	if err != nil {
		if f != nil {
			_ = f.Close()
		}
		return nil, nil, err
	}
	f.Path = filename

	if glog.V(3).Enabled() {
		glog.Infof("Read Excel File with Sheets %v", f.GetSheetList())
	}
	keepTmpDir = true
	return f, func() {
		_ = f.Close()
		_ = os.RemoveAll(tmpDir)
	}, nil
}

type MeterCodeType int

const (
	Total MeterCodeType = iota
	Share
	Coverage
	Profit
	TotalTF
	ShareTF
	CoverageTF
	ProfitTF
	TotalProd
	Bad
)

type excelHeader struct {
	meteringPointId map[int]string
	energyDirection map[int]model.MeterDirection
	periodStart     map[int]string
	periodEnd       map[int]string
	meterCode       map[int]MeterCodeType
}

type excelCounterPointMeta struct {
	*model.CounterPointMeta
	Idx     int
	IdxG2   int
	IdxG3   int
	IdxG1TF int
	IdxG2TF int
	IdxG3TF int
}

func buildMatrixMetaStruct(db ebow.IBowStorage, excelHeader excelHeader) (map[int]*excelCounterPointMeta, []*model.CounterPointMeta, error) {
	glog.V(3).Info("Build Meta Matrix")

	ensureMeteringPoint := func(meter string) (string, error) {
		if len(meter) < 33 {
			return "", errors.New("not a metercode")
		}
		return meter, nil
	}

	filterExcelHeaderCol := func(pattern ...string) func(string) bool {
		return func(name string) bool {
			for _, p := range pattern {
				if name == p {
					return false
				}
			}
			return true
		}
	}

	type pair struct {
		key   string
		value int
		vG2   int
		vG3   int
		vG1TF int
		vG2TF int
		vG3TF int
		dir   model.MeterDirection
		end   string
		begin string
	}
	msSet := map[string]pair{}
	meteringIdSet := map[string]int{}
	headerFilter := filterExcelHeaderCol("MM", "TOTAL")

	for i := 0; i < len(excelHeader.meteringPointId); i++ {
		if headerFilter(strings.ToUpper(excelHeader.meteringPointId[i])) {
			if i < len(excelHeader.meterCode) {
				if v, err := ensureMeteringPoint(excelHeader.meteringPointId[i]); err == nil {
					if excelHeader.meterCode[i] == Total {
						if _, ok := meteringIdSet[v]; !ok && strings.ToLower(v) != "total" {
							meteringIdSet[v] = i
							if _ms, ok := msSet[v]; ok {
								_ms.value = i
								msSet[v] = _ms
							} else {
								msSet[v] = pair{v, i, -1, -1, -1, -1, -1, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
							}
						}
					} else if strings.ToLower(v) != "total" && (excelHeader.meterCode[i] == Share || excelHeader.meterCode[i] == Profit) {
						if _ms, ok := msSet[v]; ok {
							_ms.vG2 = i
							msSet[v] = _ms
						} else {
							msSet[v] = pair{v, -1, i, -1, -1, -1, -1, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
						}
					} else if strings.ToLower(v) != "total" && excelHeader.meterCode[i] == Coverage {
						if _ms, ok := msSet[v]; ok {
							_ms.vG3 = i
							msSet[v] = _ms
						} else {
							msSet[v] = pair{v, -1, -1, i, -1, -1, -1, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
						}
					} else if strings.ToLower(v) != "total" && excelHeader.meterCode[i] == TotalTF {
						if _ms, ok := msSet[v]; ok {
							_ms.vG1TF = i
							msSet[v] = _ms
						} else {
							msSet[v] = pair{v, -1, -1, -1, i, -1, -1, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
						}
					} else if strings.ToLower(v) != "total" && excelHeader.meterCode[i] == ProfitTF {
						if _ms, ok := msSet[v]; ok {
							_ms.vG2TF = i
							msSet[v] = _ms
						} else {
							msSet[v] = pair{v, -1, -1, -1, -1, i, -1, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
						}
					} else if strings.ToLower(v) != "total" && excelHeader.meterCode[i] == CoverageTF {
						if _ms, ok := msSet[v]; ok {
							_ms.vG3TF = i
							msSet[v] = _ms
						} else {
							msSet[v] = pair{v, -1, -1, -1, -1, -1, i, excelHeader.energyDirection[i], excelHeader.periodEnd[i], excelHeader.periodStart[i]}
						}
					}
				}
			}
		}
	}

	// Validate and Transform read metering point data
	// - Transform Meterstructure to a Map, SortedList
	// - Check for valid period date
	ms := []pair{}
	for _, v := range msSet {
		if !(v.value < 0) {
			// check date
			_, err := utils.ParseTime(v.end, 0)
			if err != nil {
				return nil, nil, errors.New("'Period End' date missing or wrong format")
			}
			_, err = utils.ParseTime(v.begin, 0)
			if err != nil {
				return nil, nil, errors.New("'Period Start' date missing or wrong format")
			}

			// Setup Map
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
			switch kv.dir {
			case model.CONSUMER_DIRECTION:
				metaInfo.ConsumerCount += 1
				metaInfo.MaxConsumerIdx += 1
				storedCpMeta[meterpoint] = &model.CounterPointMeta{
					ID:          fmt.Sprintf("%.3d", len(storedCpMeta)),
					SourceIdx:   metaInfo.MaxConsumerIdx,
					Name:        meterpoint,
					Dir:         model.CONSUMER_DIRECTION,
					PeriodStart: kv.begin,
					PeriodEnd:   kv.end,
				}
			case model.PRODUCER_DIRECTION:
				metaInfo.ProducerCount += 1
				metaInfo.MaxProducerIdx += 1
				storedCpMeta[meterpoint] = &model.CounterPointMeta{
					ID:          fmt.Sprintf("%.3d", len(storedCpMeta)),
					SourceIdx:   metaInfo.MaxProducerIdx,
					Name:        meterpoint,
					Dir:         model.PRODUCER_DIRECTION,
					PeriodStart: kv.begin,
					PeriodEnd:   kv.end,
				}
			default:
				return nil, nil, fmt.Errorf("'Energy direction' missing or unknown for metering point %s: %q", meterpoint, kv.dir)
			}
		}
		storedMeta := storedCpMeta[kv.key]
		nStoredPeriodEnd, _ := utils.ParseTime(storedMeta.PeriodEnd, 0)
		nExcelPeriodEnd, _ := utils.ParseTime(kv.end, time.Now().UnixMilli())
		if nExcelPeriodEnd.Unix() > nStoredPeriodEnd.Unix() {
			storedMeta.PeriodEnd = kv.end
		}

		nStoredPeriodStart, _ := utils.ParseTime(storedMeta.PeriodStart, time.Now().UnixMilli())
		nExcelPeriodStart, _ := utils.ParseTime(kv.begin, time.Now().UnixMilli())
		if nStoredPeriodStart.Unix() > nExcelPeriodStart.Unix() {
			storedMeta.PeriodStart = kv.begin
		}

		excelCpMeta[i] = &excelCounterPointMeta{CounterPointMeta: storedMeta}
		switch kv.dir {
		case model.PRODUCER_DIRECTION:
			excelCpMeta[i].Idx = kv.value
			excelCpMeta[i].IdxG2 = kv.vG2
			excelCpMeta[i].IdxG3 = kv.vG3
			excelCpMeta[i].IdxG1TF = kv.vG1TF
			excelCpMeta[i].IdxG2TF = kv.vG2TF
			excelCpMeta[i].IdxG3TF = kv.vG3TF
		default:
			excelCpMeta[i].Idx = kv.value
			excelCpMeta[i].IdxG2 = kv.vG2
			excelCpMeta[i].IdxG3 = kv.vG3
			excelCpMeta[i].IdxG1TF = kv.vG1TF
			excelCpMeta[i].IdxG2TF = kv.vG2TF
			excelCpMeta[i].IdxG3TF = kv.vG3TF
		}
	}

	updateCpMeta := []*model.CounterPointMeta{}
	for _, v := range storedCpMeta {
		updateCpMeta = append(updateCpMeta, v)
	}

	sort.Slice(updateCpMeta, func(i, j int) bool {
		return updateCpMeta[i].SourceIdx < updateCpMeta[j].SourceIdx
	})
	glog.V(4).Info("ExcelMeta:")
	for k, v := range excelCpMeta {
		glog.V(4).Infof("Key: %d Value: %s %s\n", k, v.CounterPointMeta.ID, v.CounterPointMeta.Dir)
	}
	glog.V(4).Info("UpdateMeta:")
	for i, v := range updateCpMeta {
		glog.V(4).Infof("Idx: %d Value: %s %s\n", i, v.ID, v.Dir)
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

// parseExcelFloat reads a value cell: surrounding whitespace (also a non-breaking space) is ignored, empty
// is 0, a decimal comma ("1,5") is accepted. NaN and infinities are refused.
func parseExcelFloat(c string) (float64, error) {
	c = strings.TrimSpace(c) // unicode.IsSpace: includes U+00A0
	if len(c) == 0 {
		return 0, nil
	}
	if strings.Contains(c, ",") && !strings.Contains(c, ".") {
		c = strings.Replace(c, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(c, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("not a finite number: %q", c)
	}
	return f, nil
}

func returnFloat(c string) float64 {
	f, err := parseExcelFloat(c)
	if err != nil {
		return 0
	}
	return f
}

// checkMeterValues refuses a data row with a value cell that is not a number; returnMeterValue
// would read it as 0.
func checkMeterValues(cols []string, excelCpMeta map[int]*excelCounterPointMeta) error {
	for _, v := range excelCpMeta {
		for _, idx := range []int{v.Idx, v.IdxG2, v.IdxG3, v.IdxG1TF, v.IdxG2TF, v.IdxG3TF} {
			if idx < 0 || len(cols) <= (idx+1) {
				continue
			}
			if _, err := parseExcelFloat(cols[idx+1]); err != nil {
				return fmt.Errorf("row %s, metering point %s: value %q is not a number", cols[0], v.Name, cols[idx+1])
			}
		}
	}
	return nil
}

func returnMeterValue(cols []string, idx int) float64 {
	if idx < 0 || len(cols) <= (idx+1) {
		return 0
	}
	return returnFloat(cols[idx+1])
}

func returnMeterCode(c string) MeterCodeType {
	switch {
	case strings.Contains(c, "GESAMTVERBRAUCH LT. MESSUNG (BEI TEILNAHME GEM. ERZEUGUNG)"):
		return Total // G1 Consumer
	case strings.Contains(c, "VERBRAUCH LT. MESSUNG ENTSPRECHEND DEM TEILNAHMEFAKTOR JE ZP UND EC-ID"):
		return TotalTF
	case strings.Contains(c, "ANTEIL GEMEINSCHAFTLICHE ERZEUGUNG"):
		return Share // G2 Consumer
	case strings.Contains(c, "EIGENDECKUNG GEMEINSCHAFTLICHE ERZEUGUNG"):
		return Coverage // G3 Consumer
	case strings.Contains(c, "EIGENDECKUNG AUS ERNEUERBARER ENERGIE"):
		return CoverageTF // G3 Consumer
	case strings.Contains(c, "GESAMT/ÜBERSCHUSSERZEUGUNG, GEMEINSCHAFTSÜBERSCHUSS"):
		return Profit // G2 Producer
	case strings.Contains(c, "RESTÜBERSCHUSS BEI EG UND JE ZP"):
		return ProfitTF // G2 Producer
	case strings.Contains(c, "GESAMTE GEMEINSCHAFTLICHE ERZEUGUNG [KWH]"):
		return Total // G1 Producer
	case strings.Contains(c, "ERZEUGUNG LT. MESSUNG ENTSPRECHEND DEM TEILNAHMEFAKTOR UND EC-ID"):
		return TotalTF // G1 Producer
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

func isDate(cell string) bool {
	if len(cell) > 0 {
		if numberPattern.MatchString(cell) {
			return true
		}

		if dateLine.MatchString(cell) {
			return true
		}
	}
	return false
}

func isDateString(cell string) bool {
	if dateLine.MatchString(cell) {
		return true
	}
	return false
}

func getExcelDate(cell string) (int, int, int, int, int, int) {
	excelTime := parseExcelDate(cell).Round(15 * time.Minute)
	return excelTime.Day(), int(excelTime.Month()), excelTime.Year(), excelTime.Hour(), excelTime.Minute(), excelTime.Second()
}

func parseExcelDate(cell string) time.Time {
	if isDateString(cell) {
		return utils.StringToTime(cell, time.Now())
	} else {
		var excelEpoch = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)
		var days, _ = strconv.ParseFloat(cell, 64)
		return excelEpoch.Add(time.Second * time.Duration(days*86400))
	}
}

func excelDateToString(cell string) string {
	d, m, y, hh, mm, ss := getExcelDate(cell)
	return fmt.Sprintf("%.2d.%.2d.%.4d %.2d:%.2d:%.2d", d, m, y, hh, mm, ss)
}
