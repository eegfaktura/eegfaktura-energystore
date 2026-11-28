package json

import (
	"encoding/json"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow/codec"
)

type Codec struct{}

func (c Codec) Marshal(v interface{}, in []byte) (out []byte, err error) {
	return json.Marshal(v)
}

// Unmarshal decodes an energy row with decodeRawSourceLine, which yields what encoding/json yields; any
// other value, and a row that decoder does not accept, goes to encoding/json.
func (c Codec) Unmarshal(data []byte, v interface{}) error {
	if r, ok := v.(*model.RawSourceLine); ok && r != nil && decodeRawSourceLine(data, r) {
		return nil
	}
	return json.Unmarshal(data, v)
}

func (c Codec) Format() codec.Format {
	return codec.JSON
}
