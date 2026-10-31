package rest_test

import (
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"github.com/stretchr/testify/assert"
)

// 261008-ca3: a GEA sub-community (tenant = RC number + -00n) reads the energy stored under its RC number.
const subTenant = tenant + "-001"

func rawOf(header map[string]string) apitest.Request {
	return apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/raw", Header: header,
		Body: fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, ms(day), ms(day.Add(23*time.Hour+45*time.Minute)))}
}

func TestSubCommunityTenantReadsTheStemStore(t *testing.T) {
	seed(t)
	before := dirs(t)
	rec := apitest.Do(apitest.Router(), rawOf(map[string]string{
		"Authorization": "Bearer " + apitest.AppToken([]string{subTenant}), "tenant": subTenant}))
	assert.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), meter)
	assert.Equal(t, before, dirs(t), "no directory for the sub-community")
}

// The check of the header against the token stays exact: a token of one sub-community does not pass as a
// sibling or as the stem.
func TestSubCommunityTokenDoesNotPassAsOtherTenant(t *testing.T) {
	seed(t)
	token := "Bearer " + apitest.AppToken([]string{subTenant})
	for _, other := range []string{tenant + "-002", tenant} {
		rec := apitest.Do(apitest.Router(), rawOf(map[string]string{"Authorization": token, "tenant": other}))
		assert.Contains(t, []int{401, 403}, rec.Code, "tenant %s: %s", other, rec.Body.String())
	}
}
