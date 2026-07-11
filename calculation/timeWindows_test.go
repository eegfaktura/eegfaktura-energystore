package calculation

import (
	"testing"
	"time"

	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestParseWindowTime(t *testing.T) {
	m, err := parseWindowTime("06:15")
	require.NoError(t, err)
	assert.Equal(t, 6*60+15, m)

	_, err = parseWindowTime("06:07")
	assert.Error(t, err, "not on 15-min raster")

	_, err = parseWindowTime("24:00")
	assert.Error(t, err)

	_, err = parseWindowTime("nonsense")
	assert.Error(t, err)
}

func TestTimeWindowContains(t *testing.T) {
	day, err := parseTimeWindow(model.TimeWindow{Key: "T1", From: "06:00", To: "08:00"})
	require.NoError(t, err)
	assert.True(t, day.contains(6*60), "from inclusive")
	assert.True(t, day.contains(7*60+45))
	assert.False(t, day.contains(8*60), "to exclusive")
	assert.False(t, day.contains(5*60+45))

	night, err := parseTimeWindow(model.TimeWindow{Key: "T2", From: "20:00", To: "06:00"})
	require.NoError(t, err)
	assert.True(t, night.contains(20*60), "from inclusive")
	assert.True(t, night.contains(23*60+45))
	assert.True(t, night.contains(0))
	assert.True(t, night.contains(5*60+45))
	assert.False(t, night.contains(6*60), "to exclusive")
	assert.False(t, night.contains(12*60))
}

func TestValidateTimeWindows(t *testing.T) {
	mkParticipants := func(windows []model.TimeWindow) []model.ParticipantReport {
		return []model.ParticipantReport{{
			ParticipantId: "P1",
			Meters:        []*model.MeterReport{{MeterId: "M1", TimeWindows: windows}},
		}}
	}

	assert.NoError(t, ValidateTimeWindows(mkParticipants(nil)))
	assert.NoError(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "T1", From: "06:00", To: "08:00"},
		{Key: "T2", From: "20:00", To: "06:00"},
	})))

	assert.Error(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "T1", From: "06:00", To: "08:00"},
		{Key: "T1", From: "10:00", To: "11:00"},
	})), "duplicate key")

	assert.Error(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "BASE", From: "06:00", To: "08:00"},
	})), "invalid key")

	assert.Error(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "T1", From: "06:00", To: "06:00"},
	})), "from == to")

	assert.Error(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "T1", From: "06:05", To: "08:00"},
	})), "raster")

	assert.Error(t, ValidateTimeWindows(mkParticipants([]model.TimeWindow{
		{Key: "T1", From: "06:00", To: "07:00"},
		{Key: "T2", From: "08:00", To: "09:00"},
		{Key: "T1", From: "10:00", To: "11:00"},
	})), "more than 2 windows")
}

// TestCalcParticipantReportBuckets folds a hand-computable quarter-hour series
// into time-of-use buckets: consumer with T1 (06:00-08:00) + midnight-crossing
// T2 (20:00-06:00), producer with T1 only. BASE must be the exact residual.
func TestCalcParticipantReportBuckets(t *testing.T) {
	// line layout: Consumers = [consumption, allocation, utilization] per
	// consumer; Producers = [production, allocation] per producer.
	mkLine := func(id string, util, prod, dist float64) *model.RawSourceLine {
		return &model.RawSourceLine{
			Id:        id,
			Consumers: []float64{util, 0, util},
			Producers: []float64{prod, dist},
		}
	}

	iter := &mocks.MockBowRange{Entries: []*model.RawSourceLine{
		mkLine("CP/2024/01/01/05/45/00", 7, 0, 0),  // T2 (morning side of midnight window)
		mkLine("CP/2024/01/01/06/00/00", 2, 10, 4), // T1 (from inclusive)
		mkLine("CP/2024/01/01/07/45/00", 3, 0, 0),  // T1
		mkLine("CP/2024/01/01/08/00/00", 4, 0, 0),  // BASE (to exclusive)
		mkLine("CP/2024/01/01/12/00/00", 1, 20, 5), // BASE
		mkLine("CP/2024/01/01/20/00/00", 5, 0, 0),  // T2 (from inclusive)
		mkLine("CP/2024/01/01/23/45/00", 6, 0, 0),  // T2
		mkLine("CP/2024/01/02/00/00/00", 8, 0, 0),  // T2, next day
	}}
	iter.On("Next", mock.Anything).Return(true)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli()

	report := model.ReportResponse{ParticipantReports: []model.ParticipantReport{
		{
			ParticipantId: "P1",
			Meters: []*model.MeterReport{{
				MeterId: "CONS1", From: from, Until: until,
				TimeWindows: []model.TimeWindow{
					{Key: "T2", From: "20:00", To: "06:00"},
					{Key: "T1", From: "06:00", To: "08:00"},
				},
			}},
		},
		{
			ParticipantId: "P2",
			Meters: []*model.MeterReport{{
				MeterId: "PROD1", From: from, Until: until,
				TimeWindows: []model.TimeWindow{
					{Key: "T1", From: "06:00", To: "08:00"},
				},
			}},
		},
	}}
	reportValues := ConvertToMeterMap(&report)

	cpMeta := map[string]*model.CounterPointMeta{
		"CONS1": {Name: "CONS1", SourceIdx: 0, Dir: model.CONSUMER_DIRECTION},
		"PROD1": {Name: "PROD1", SourceIdx: 0, Dir: model.PRODUCER_DIRECTION},
	}
	metaInfo := &model.CounterPointMetaInfo{ConsumerCount: 1, ProducerCount: 1}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
	err := calcParticipantReport(iter, &reportValues, AllocDynamicV2, cpMeta, metaInfo, "CP", startDate,
		func(currentDate time.Time) int { return currentDate.Day() })
	require.NoError(t, err)

	consumer := report.ParticipantReports[0].Meters[0]
	require.NotNil(t, consumer.Report)
	assert.Equal(t, 36.0, consumer.Report.Summary.Utilization)
	require.Len(t, consumer.Report.Buckets, 3)
	assert.Equal(t, model.Bucket{Key: "BASE", KWh: 5}, consumer.Report.Buckets[0])
	assert.Equal(t, model.Bucket{Key: "T1", KWh: 5}, consumer.Report.Buckets[1])
	assert.Equal(t, model.Bucket{Key: "T2", KWh: 26}, consumer.Report.Buckets[2])

	// exact kWh partition: sum of buckets == period total
	sum := 0.0
	for _, b := range consumer.Report.Buckets {
		sum += b.KWh
	}
	assert.Equal(t, consumer.Report.Summary.Utilization, sum)

	producer := report.ParticipantReports[1].Meters[0]
	require.NotNil(t, producer.Report)
	assert.Equal(t, 30.0, producer.Report.Summary.Production)
	assert.Equal(t, 9.0, producer.Report.Summary.Allocation)
	require.Len(t, producer.Report.Buckets, 2)
	assert.Equal(t, model.Bucket{Key: "T1", KWh: 6}, producer.Report.Buckets[1])
	assert.Equal(t, model.Bucket{Key: "BASE", KWh: 15}, producer.Report.Buckets[0])
}

// TestCalcParticipantReportNoWindows ensures the report of a meter without
// time windows carries no buckets (unchanged behaviour).
func TestCalcParticipantReportNoWindows(t *testing.T) {
	iter := &mocks.MockBowRange{Entries: []*model.RawSourceLine{
		{Id: "CP/2024/01/01/12/00/00", Consumers: []float64{1, 0, 1}, Producers: []float64{0, 0}},
	}}
	iter.On("Next", mock.Anything).Return(true)

	report := model.ReportResponse{ParticipantReports: []model.ParticipantReport{{
		ParticipantId: "P1",
		Meters: []*model.MeterReport{{
			MeterId: "CONS1",
			From:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
			Until:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
		}},
	}}}
	reportValues := ConvertToMeterMap(&report)

	cpMeta := map[string]*model.CounterPointMeta{
		"CONS1": {Name: "CONS1", SourceIdx: 0, Dir: model.CONSUMER_DIRECTION},
	}
	metaInfo := &model.CounterPointMetaInfo{ConsumerCount: 1, ProducerCount: 1}

	err := calcParticipantReport(iter, &reportValues, AllocDynamicV2, cpMeta, metaInfo, "CP",
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
		func(currentDate time.Time) int { return currentDate.Day() })
	require.NoError(t, err)

	meter := report.ParticipantReports[0].Meters[0]
	require.NotNil(t, meter.Report)
	assert.Empty(t, meter.Report.Buckets)
	assert.Equal(t, 1.0, meter.Report.Summary.Utilization)
}
