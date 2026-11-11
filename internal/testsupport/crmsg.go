package testsupport

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"at.ourproject/energystore/model"
)

// CrMsgBuilder builds a CR_MSG as eda-xp publishes it on eda/response/<tenant>/protocol/cr_msg:
// one metering point, one or more energy blocks of quarter-hour values per meter code.
type CrMsgBuilder struct {
	msg model.MqttEnergyMessage
}

// CrMsg starts a message for the metering point meter of the community ecId.
func CrMsg(ecId, meter string) *CrMsgBuilder {
	return &CrMsgBuilder{msg: model.MqttEnergyMessage{
		ConversationId: "RC100001202601010000000000000000001",
		MessageCode:    "DATEN_CRMSG",
		Meter:          model.EnergyMeter{MeteringPoint: meter},
		EcId:           ecId,
	}}
}

// Direction sets the optional direction of the metering point.
func (b *CrMsgBuilder) Direction(d model.MeterDirection) *CrMsgBuilder {
	b.msg.Meter.Direction = string(d)
	return b
}

// Energy adds one block starting at start: every series holds consecutive quarter-hour values
// with the quality method (e.g. "L1"). The block end is exclusive (the last value's "to"), as
// eda-xp sends it. All series must have the same length.
func (b *CrMsgBuilder) Energy(start time.Time, method string, series map[model.MeterCodeValue][]float64) *CrMsgBuilder {
	n := 0
	block := model.MqttEnergy{Start: start.UnixMilli()}
	codes := make([]model.MeterCodeValue, 0, len(series))
	for code := range series {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	for _, code := range codes {
		values := series[code]
		data := model.MqttEnergyData{MeterCode: code}
		for i, v := range values {
			from := start.Add(time.Duration(i) * 15 * time.Minute)
			data.Value = append(data.Value, model.MqttEnergyValue{
				From: from.UnixMilli(), To: from.Add(15 * time.Minute).UnixMilli(), Method: method, Value: v})
		}
		n = len(values)
		block.Data = append(block.Data, data)
	}
	block.End = start.Add(time.Duration(n) * 15 * time.Minute).UnixMilli()
	b.msg.Energy = append(b.msg.Energy, block)
	return b
}

// Message returns a copy of the built message (the decoded form the importer takes).
func (b *CrMsgBuilder) Message() *model.MqttEnergyMessage {
	m := b.msg
	return &m
}

// Payload returns the wire form: JSON, gzip, base64 (StdEncoding), as eda-xp sends it.
func (b *CrMsgBuilder) Payload(t testing.TB) []byte {
	t.Helper()
	raw, err := json.Marshal(b.msg)
	if err != nil {
		t.Fatalf("marshal CR_MSG: %v", err)
	}
	return EncodeCrMsg(t, raw)
}

// EncodeCrMsg wraps raw JSON in the CR_MSG transport encoding (gzip, then base64).
func EncodeCrMsg(t testing.TB, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		t.Fatalf("gzip CR_MSG: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip CR_MSG: %v", err)
	}
	return []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
}
