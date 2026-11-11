package contract_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	protoMessage = regexp.MustCompile(`^message (\w+)`)
	protoField   = regexp.MustCompile(`^\s*((?:optional|repeated)\s+)?(\w+)\s+(\w+)\s*=\s*(\d+);`)
	protoRPC     = regexp.MustCompile(`^\s*rpc (\w+) \((\w+)\) returns \((\w+)\)`)
	protoService = regexp.MustCompile(`^service (\w+)`)
)

// TestMasterdataProtoContract: protoc/masterdata.proto has the messages, fields, numbers and types of
// the backend's copy (testdata/backend/masterdata-fields.txt).
func TestMasterdataProtoContract(t *testing.T) {
	var got []string
	msg, svc := "", ""
	for _, l := range lines(t, "../protoc/masterdata.proto") {
		if m := protoService.FindStringSubmatch(l); m != nil {
			svc = m[1]
		}
		if m := protoRPC.FindStringSubmatch(l); m != nil {
			got = append(got, strings.Join([]string{"service", svc, m[1], m[2], m[3]}, " "))
		}
		if m := protoMessage.FindStringSubmatch(l); m != nil {
			msg = m[1]
		}
		if m := protoField.FindStringSubmatch(l); m != nil && msg != "" {
			got = append(got, strings.TrimSpace(strings.Join([]string{"message", msg, m[3], m[4], strings.TrimSpace(m[1]), m[2]}, " ")))
		}
	}
	var want []string
	for _, l := range lines(t, "testdata/backend/masterdata-fields.txt") {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		want = append(want, l)
	}
	normalize := func(s []string) []string {
		out := make([]string, len(s))
		for i, x := range s {
			out[i] = strings.Join(strings.Fields(x), " ")
		}
		sort.Strings(out)
		return out
	}
	assert.Equal(t, normalize(want), normalize(got), "protoc/masterdata.proto against eegfaktura-backend proto/masterdata.proto at f4974b2")
}
