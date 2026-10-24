package rest_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

const noData = "AT0030000000000000000000000000099"

func excelBody(ec string, meters ...string) string {
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	var cps []string
	for _, m := range meters {
		cps = append(cps, fmt.Sprintf(`{"meteringPoint":"%s","direction":"CONSUMPTION","name":"N","activeSince":%d,"inactiveSince":%d}`, m, start, end))
	}
	return fmt.Sprintf(`{"start":%d,"end":%d,"communityId":"%s","cps":[%s]}`, start, end, ec, strings.Join(cps, ","))
}

// A requested metering point without any stored data (no entry in cpmeta/0) made the Excel
// download panic in the header row (nil meta). It is left out, as the summary sheet already did.
func TestExcelReportSkipsMeterWithoutData(t *testing.T) {
	seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/" + ecId + "/excel/report/download",
		Header: appHeader(), Body: excelBody(ecId, meter, noData)})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	f, err := excelize.OpenReader(rec.Body)
	require.NoError(t, err)
	rows, err := f.GetRows("Energiedaten")
	require.NoError(t, err)
	require.Greater(t, len(rows), 2)
	assert.Equal(t, []string{"MeteringpointID", meter, meter, meter}, rows[1])
}

// A community without any imported data: the same 404 no_entries as a period without rows.
func TestExcelReportWithoutAnyData(t *testing.T) {
	testsupport.UseTempPersistence(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/RC100009/excel/report/download",
		Header: appHeader(), Body: excelBody("RC100009", meter)})
	require.Equal(t, 404, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":{"code":1000,"error":"no_entries","message":"no Rows found"}}`, rec.Body.String())
}
