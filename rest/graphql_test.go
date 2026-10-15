package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"testing"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// uploadBody builds a GraphQL multipart request (graphql-multipart-request-spec) for singleUpload
// with an empty workbook; the tenant argument equals the tenant header, as the web sends it.
func uploadBody(t *testing.T, tenantArg string) (io.Reader, string) {
	t.Helper()
	ops := map[string]any{
		"query":     `mutation($file: Upload!, $tenant: String!, $ecId: String!) { singleUpload(tenant: $tenant, ecId: $ecId, sheet: "Sheet1", file: $file) }`,
		"variables": map[string]any{"file": nil, "tenant": tenantArg, "ecId": ecId},
	}
	opsJSON, err := json.Marshal(ops)
	require.NoError(t, err)

	var xlsx bytes.Buffer
	f := excelize.NewFile()
	require.NoError(t, f.Write(&xlsx))

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	require.NoError(t, mw.WriteField("operations", string(opsJSON)))
	require.NoError(t, mw.WriteField("map", `{"0":["variables.file"]}`))
	part, err := mw.CreateFormFile("0", "upload.xlsx")
	require.NoError(t, err)
	_, err = part.Write(xlsx.Bytes())
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &body, mw.FormDataContentType()
}

type gqlResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func TestGraphQLLastEnergyDate(t *testing.T) {
	seed(t)
	h := apitest.Router()
	rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query",
		Header: map[string]string{"Authorization": "Bearer " + apitest.AppToken([]string{tenant}), "tenant": tenant},
		Body:   fmt.Sprintf(`{"query":"{ lastEnergyDate(tenant: \"%s\", ecId: \"%s\") }"}`, tenant, ecId)})
	require.Equal(t, 200, rec.Code)
	var resp gqlResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Errors)
	assert.Equal(t, "01.06.2026 23:45:00", resp.Data["lastEnergyDate"], "the period end of the stored meta")
}

// An upload without energy data must be refused; today it imports zero meters and reports success
// (known-errors #28, F13: "a sheet without the Metercode header imports zero meters as success").
func TestGraphQLUploadOfAnEmptyWorkbook(t *testing.T) {
	t.Skip("known-errors #28")
	seed(t)
	h := apitest.Router()
	body, ct := uploadBody(t, tenant)
	rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query", RawBody: body, ContentType: ct,
		Header: map[string]string{"Authorization": "Bearer " + apitest.AppToken([]string{tenant}), "tenant": tenant}})
	require.Equal(t, 200, rec.Code)
	var resp gqlResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Errors, "a workbook without the energy sheet is an error, not an import")
}
