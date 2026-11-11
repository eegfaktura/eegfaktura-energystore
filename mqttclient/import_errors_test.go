package mqttclient

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// days builds one block of n consecutive days for meter (values 0.01·(slot+1)).
func days(n int) *model.MqttEnergyMessage {
	v := make([]float64, 96*n)
	for i := range v {
		v[i] = float64(i%96+1) / 100
	}
	return testsupport.CrMsg(ackEcId, ackMeter).Direction(model.CONSUMER_DIRECTION).
		Energy(ackDay, "L1", map[model.MeterCodeValue][]float64{model.CODE_CON: v}).Message()
}

// An error of the meta record affects every day: the import stops at the first one (review of #85).
func TestImportStopsAtMetaRecordError(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	corruptMeta(t, dir, ackTenant, ackEcId)
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
	defer imp.Close()

	err := imp.Import(days(3))
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrMetaRecord), "got %v", err)
	var ie *importErrors
	require.True(t, errors.As(err, &ie))
	assert.Len(t, ie.errs, 1, "no further day tried")
	assert.Zero(t, ie.omitted)
}

// Errors of single days are collected, at most maxImportErrors of them in the text, on one line.
func TestImportErrorsAreCappedOnOneLine(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	db, err := ebow.Open(filepath.Join(dir, strings.ToLower(ackTenant), ackEcId))
	require.NoError(t, err)
	type badLine struct {
		Id        string `bow:"key"`
		Consumers string
	}
	for d := 1; d <= 7; d++ {
		require.NoError(t, db.Bucket("rawdata").Put(badLine{Id: fmt.Sprintf("CP/2026/06/%02d/12/00/00", d), Consumers: "garbage"}))
	}
	require.NoError(t, db.Close())

	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
	defer imp.Close()
	err = imp.Import(days(7))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)
	assert.NotContains(t, err.Error(), "\n")
	assert.Contains(t, err.Error(), "… 2 more")
	assert.Equal(t, maxImportErrors, strings.Count(err.Error(), "stored record cannot be read"))
}
