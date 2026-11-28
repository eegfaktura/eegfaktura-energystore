package rest_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/rest"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// served wraps the router as server.go does (without CORS).
func served() http.Handler {
	return rest.HTTPHandler(apitest.Router())
}

// gunzip checks the answer is gzipped and returns the decoded body.
func gunzip(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Values("Vary"), "Accept-Encoding")
	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	return string(body)
}

func withGzip(h map[string]string) map[string]string {
	h["Accept-Encoding"] = "gzip, deflate, br"
	return h
}

// A JSON answer is gzipped for a client that accepts it and plain for one that does not; the
// decoded body is the plain one.
func TestCompressJSONAnswer(t *testing.T) {
	seed(t)
	req := apitest.Request{Method: "GET", Path: "/eeg/" + ecId + "/lastRecordDate", Header: appHeader()}
	plain := apitest.Do(served(), req)
	require.Equal(t, 200, plain.Code, plain.Body.String())
	assert.Empty(t, plain.Header().Get("Content-Encoding"))
	assert.Contains(t, plain.Header().Values("Vary"), "Accept-Encoding")
	assert.Contains(t, plain.Body.String(), `"periodEnd"`)

	req.Header = withGzip(appHeader())
	rec := apitest.Do(served(), req)
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, plain.Body.String(), gunzip(t, rec))
}

// The Basic-auth API (/query/rawdata, external API applications) is compressed too.
func TestCompressJSONQueryRawData(t *testing.T) {
	seed(t)
	req := apitest.Request{Method: "POST", Path: "/query/rawdata", Header: apiHeader(),
		Body: fmt.Sprintf(`{"cps":[{"meteringPoint":"%s"}],"ecId":"%s","start":%d,"end":%d}`, meter, ecId, ms(day), ms(day.Add(time.Hour)))}
	plain := apitest.Do(served(), req)
	require.Equal(t, 200, plain.Code, plain.Body.String())
	req.Header = withGzip(apiHeader())
	rec := apitest.Do(served(), req)
	require.Equal(t, 200, rec.Code)
	assert.JSONEq(t, plain.Body.String(), gunzip(t, rec))
}

// The Excel download stays uncompressed even when the client accepts gzip.
func TestCompressJSONSkipsExcelDownload(t *testing.T) {
	seed(t)
	rec := apitest.Do(served(), apitest.Request{Method: "POST", Path: "/eeg/" + ecId + "/excel/report/download",
		Header: withGzip(appHeader()), Body: excelBody(ecId, meter)})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	_, err := excelize.OpenReader(rec.Body)
	assert.NoError(t, err)
}

// A panicking handler still gets the access log's 500 when the client accepts gzip: the compression
// sits outside logging.AccessLog in rest.HTTPHandler.
func TestCompressJSONPanicIs500(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := apitest.Do(rest.HTTPHandler(r),
		apitest.Request{Method: "GET", Path: "/boom", Header: withGzip(map[string]string{})})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
