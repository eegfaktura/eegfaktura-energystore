package model

import (
	"at.ourproject/energystore/model"
	"encoding/json"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"io"
	"log"
)

func UnmarshalEegEnergy(v interface{}) (model.EegEnergy, error) {
	byteData, err := json.Marshal(v)
	if err != nil {
		return model.EegEnergy{}, fmt.Errorf("FAIL WHILE MARSHAL SCHEME")
	}
	tmp := model.EegEnergy{}
	err = json.Unmarshal(byteData, &tmp)
	if err != nil {
		return model.EegEnergy{}, fmt.Errorf("FAIL WHILE UNMARSHAL SCHEME")
	}
	return tmp, nil
}

func MarshalEegEnergy(e model.EegEnergy) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		byteData, err := json.Marshal(e)
		if err != nil {
			log.Printf("FAIL WHILE MARSHAL JSON %v\n", string(byteData))
		}
		_, err = w.Write(byteData)
		if err != nil {
			log.Printf("FAIL WHILE WRITE DATA %v\n", string(byteData))
		}
	})
}
