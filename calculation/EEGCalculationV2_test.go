package calculation

import (
	"at.ourproject/energystore/excel"
	model "at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCalculateBiAnnualParticipantReport(t *testing.T) {

	db, err := store.OpenStorageTest("excelsource", "../test/rawdata")
	require.NoError(t, err)
	defer func() {
		db.Close()
		//os.RemoveAll("../test/rawdata/excelsource")
	}()

	excelFile, err := excel.OpenExceFile("../test/zaehlpunkte-beispieldatei-neu.xlsx")
	require.NoError(t, err)
	defer excelFile.Close()

	err = excel.ImportExcelEnergyFileNew(excelFile, "Energiedaten", db)
	require.NoError(t, err)

	var report model.ReportResponse

	clearParticipants := func() {
		report = model.ReportResponse{ParticipantReports: []model.ParticipantReport{model.ParticipantReport{
			ParticipantId: "Participant01",
			Meters: []*model.MeterReport{
				&model.MeterReport{
					MeterId: "AT003000000000000000000Zaehlpkt02",
					From:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
					Until:   time.Date(2021, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
					Report:  nil,
				},
			},
		},
			model.ParticipantReport{
				ParticipantId: "Participant02",
				Meters: []*model.MeterReport{
					&model.MeterReport{
						MeterId: "AT003000000000000000000Zaehlpkt01",
						From:    time.Date(2021, 1, 0, 0, 0, 0, 0, time.Local).UnixMilli(),
						Until:   time.Date(2021, 12, 32, 0, 0, 0, 0, time.Local).UnixMilli(),
						Report:  &model.Report{},
					},
				},
			},
			model.ParticipantReport{
				ParticipantId: "Participant03",
				Meters: []*model.MeterReport{
					&model.MeterReport{
						MeterId: "AT003000000000000000000Zaehlpkt03",
						From:    time.Date(2021, 1, 0, 0, 0, 0, 0, time.Local).UnixMilli(),
						Until:   time.Date(2021, 12, 32, 0, 0, 0, 0, time.Local).UnixMilli(),
						Report:  &model.Report{},
					},
				},
			},
		},
		}
	}

	t.Run("Monthly Calculation", func(t *testing.T) {
		clearParticipants()
		startTime := time.Now()
		require.NoError(t, CalculateMonthlyPeriodV2(db, &report, AllocDynamicV2, 2023, 7))
		fmt.Printf("-------------------------- Duration %s --------------------------\n", time.Now().Sub(startTime).Abs().String())

		participant := report.ParticipantReports[0]
		require.NotNil(t, participant.Meters[0].Report)
		assert.Equal(t, 31, len(participant.Meters[0].Report.Intermediate.Consumption), "Participant01")

		fmt.Println("REPORT PARTICIPANTS")
		participant = report.ParticipantReports[1]
		require.Equal(t, len(participant.Meters), 1)
		require.NotNil(t, participant.Meters[0].Report)
		require.Equal(t, 599.2975, participant.Meters[0].Report.Summary.Consumption)
		require.Equal(t, 32.822976, participant.Meters[0].Report.Summary.Allocation)
		require.Equal(t, 32.554322, participant.Meters[0].Report.Summary.Utilization)

		//require.Equal(t, len(participant[0].Report.Intermediate), 31)
		//require.Equal(t, 19.429, participant[0].Report.Intermediate[0].Consumption)
		//require.Equal(t, 18.316, participant[0].Report.Intermediate[30].Consumption)

		for _, v := range report.ParticipantReports {
			fmt.Printf("[%s]", v.ParticipantId)
			for _, p := range v.Meters {
				fmt.Printf("VALUES: %+v\n", p.Report)
			}
		}

	})

	t.Run("BiAnnual Calculation", func(t *testing.T) {
		clearParticipants()
		startTime := time.Now()
		var err error
		err = CalculateBiAnnualPeriodV2(db, &report, AllocDynamicV2, 2021, 1)
		require.NoError(t, err)
		fmt.Printf("-------------------------- Duration %s --------------------------\n", time.Now().Sub(startTime).Abs().String())

		participant := report.ParticipantReports[0]
		assert.Equal(t, len(participant.Meters), 1)
		require.NotNil(t, participant.Meters[0].Report)
		assert.Equal(t, 27, len(participant.Meters[0].Report.Intermediate.Utilization), "Participant01")

		fmt.Println("REPORT PARTICIPANTS")
		participant = report.ParticipantReports[1]
		require.Equal(t, len(participant.Meters), 1)
		require.Equal(t, 3372.140750, participant.Meters[0].Report.Summary.Consumption)
		require.Equal(t, 2433.459794, participant.Meters[0].Report.Summary.Allocation)
		require.Equal(t, 1388.875928, participant.Meters[0].Report.Summary.Utilization)
		require.Equal(t, 6746.79875, utils.RoundToFixed(report.TotalConsumption, 6))

		//require.Equal(t, 27, len(participant[0].Report.Intermediate))
		//require.Equal(t, 56.1225, participant[0].Report.Intermediate[0].Consumption)
		//require.Equal(t, 5.652446, participant[0].Report.Intermediate[0].Utilization)
		//require.Equal(t, 52.3905, participant[0].Report.Intermediate[26].Consumption)
		//require.Equal(t, 84.927998, participant[0].Report.Intermediate[26].Allocation)

		for _, v := range report.ParticipantReports {
			fmt.Printf("[%s]", v.ParticipantId)
			for _, p := range v.Meters {
				fmt.Printf("VALUES: %+v\n", p.Report)
			}
		}

		j, err := json.MarshalIndent(report.ParticipantReports, "", "  ")
		require.NoError(t, err)
		fmt.Printf("JSON: %s\n", string(j))
	})

	t.Run("BiAnnual Calculation divided ZPs", func(t *testing.T) {
		clearParticipants()

		report.ParticipantReports[0].Meters = append(report.ParticipantReports[0].Meters, &model.MeterReport{
			MeterId: "AT003000000000000000000Zaehlpkt01",
			From:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
			Until:   time.Date(2021, 3, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
			Report:  &model.Report{},
		})
		report.ParticipantReports[1].Meters[0].From = time.Date(2021, 4, 1, 0, 0, 0, 0, time.Local).UnixMilli()

		startTime := time.Now()
		var err error
		err = CalculateBiAnnualPeriodV2(db, &report, AllocDynamicV2, 2021, 1)
		require.NoError(t, err)
		fmt.Printf("-------------------------- Duration %s --------------------------\n", time.Now().Sub(startTime).Abs().String())

		participant := report.ParticipantReports[0]
		assert.Equal(t, len(participant.Meters), 2)
		//assert.Equal(t, 27, len(participant[0].Report.Intermediate), "Participant01")

		fmt.Println("REPORT PARTICIPANTS")
		participant = report.ParticipantReports[1]
		require.Equal(t, len(participant.Meters), 1)
		require.Equal(t, 1599.252, participant.Meters[0].Report.Summary.Consumption)
		require.Equal(t, 1860.957735, participant.Meters[0].Report.Summary.Allocation)
		require.Equal(t, 941.826526, participant.Meters[0].Report.Summary.Utilization)

		//require.Equal(t, 27, len(participant[0].Report.Intermediate))
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[0].Consumption)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[0].Utilization)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[12].Consumption)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[12].Utilization)
		//require.Equal(t, 73.39975, participant[0].Report.Intermediate[13].Consumption)
		//require.Equal(t, 41.587988, participant[0].Report.Intermediate[13].Utilization)
		//require.Equal(t, 52.3905, participant[0].Report.Intermediate[26].Consumption)
		//require.Equal(t, 84.927998, participant[0].Report.Intermediate[26].Allocation)

		for _, v := range report.ParticipantReports {
			fmt.Printf("[%s]", v.ParticipantId)
			for _, p := range v.Meters {
				fmt.Printf("VALUES: %+v\n", p.Report)
			}
		}

	})

	t.Run("BiAnnual Calculation - Production", func(t *testing.T) {
		clearParticipants()

		report = model.ReportResponse{ParticipantReports: []model.ParticipantReport{model.ParticipantReport{
			ParticipantId: "Participant01",
			Meters: []*model.MeterReport{
				&model.MeterReport{
					MeterId: "AT0020000000000000000000100193699",
					From:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
					Until:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
					Report:  nil,
				},
			},
		}}}

		startTime := time.Now()
		var err error
		err = CalculateBiAnnualPeriodV2(db, &report, AllocDynamicV2, 2023, 2)
		require.NoError(t, err)
		fmt.Printf("-------------------------- Duration %s --------------------------\n", time.Now().Sub(startTime).Abs().String())

		fmt.Println("REPORT PARTICIPANTS")
		participant := report.ParticipantReports[0]
		require.Equal(t, len(participant.Meters), 1)
		assert.Equal(t, 192.906, participant.Meters[0].Report.Summary.Production)
		assert.Equal(t, 189.738546, participant.Meters[0].Report.Summary.Allocation)

		//require.Equal(t, 27, len(participant[0].Report.Intermediate))
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[0].Consumption)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[0].Utilization)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[12].Consumption)
		//require.Equal(t, float64(0), participant[0].Report.Intermediate[12].Utilization)
		//require.Equal(t, 73.39975, participant[0].Report.Intermediate[13].Consumption)
		//require.Equal(t, 41.587988, participant[0].Report.Intermediate[13].Utilization)
		//require.Equal(t, 52.3905, participant[0].Report.Intermediate[26].Consumption)
		//require.Equal(t, 84.927998, participant[0].Report.Intermediate[26].Allocation)

		for _, v := range report.ParticipantReports {
			fmt.Printf("[%s]", v.ParticipantId)
			for _, p := range v.Meters {
				fmt.Printf("VALUES: %+v\n", p.Report)
			}
		}

	})
}
