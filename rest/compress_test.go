package rest_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/rest"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// served wraps the router as server.go does (without CORS).
func served() http.Handler {
	r := apitest.Router()
	return rest.CompressJSON(r, logging.AccessLog(r))
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
	assert.Equal(t, "Accept-Encoding", plain.Header().Get("Vary"))
	assert.Contains(t, plain.Body.String(), `"periodEnd"`)

	req.Header = withGzip(appHeader())
	rec := apitest.Do(served(), req)
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.JSONEq(t, plain.Body.String(), string(body))
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
// sits outside logging.AccessLog.
func TestCompressJSONPanicIs500(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := apitest.Do(rest.CompressJSON(r, logging.AccessLog(r)),
		apitest.Request{Method: "GET", Path: "/boom", Header: withGzip(map[string]string{})})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
