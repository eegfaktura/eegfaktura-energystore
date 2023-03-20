package calculation

import (
	"at.ourproject/energystore/model"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAllocDynamic(t *testing.T) {
	line := &model.RawSourceLine{Consumers: make([]float64, 20), Producers: make([]float64, 1)}
	line.Id = fmt.Sprintf("CP/%d/%d/%d/%d/%d/%d", 2021, 01, 10, 0, 0, 0)
	for k := 0; k < 20; k++ {
		line.Consumers[k] = float64(1000 + (k * 21))
	}
	line.Producers[0] = 35000

	m := AllocDynamic1(line)

	v1 := m.GetElm(10, 0)

	assert.Equal(t, 20, m.CountRows())
	assert.Equal(t, 1, m.CountCols())

	cm := model.MakeMatrix(line.Consumers, 20, 1)
	rm := model.Merge(m, cm)

	v2 := rm.GetElm(10, 0)

	assert.Equal(t, 20, rm.CountRows())
	assert.Equal(t, 2, rm.CountCols())
	assert.Equal(t, v1, v2)

	//fmt.Printf("Calc Matrix: %+v\n", m)
	//r := model.Multiply(m, model.MakeMatrix([]float64{1, 0 }, 1, 2))
	//
	//fmt.Printf("Result Matrix: %+v\n", r)
	//for i:=0; i < 20; i++ {
	//	fmt.Printf("Result Matrix Element 2, 1: %+v\n", r.GetElm(i,1))
	//}
	//
	//fmt.Printf("Matrix (0,0): %+v\n", r.GetElm(0,0))
	//fmt.Printf("Matrix (1,0): %+v\n", r.GetElm(1,0))
	//fmt.Printf("Matrix (0,1): %+v\n", r.GetElm(0,1))
	//
	//
	//tm := model.MakeMatrix([]float64{1, 2, 3, 4 }, 2, 2)
	//fmt.Printf("TM-Matrix: %+v\n", tm)
	//fmt.Printf("TM-Matrix (0,0): %+v\n", tm.GetElm(0,0))
	//fmt.Printf("TM-Matrix (1,0): %+v\n", tm.GetElm(1,0))
	//fmt.Printf("TM-Matrix (0,1): %+v\n", tm.GetElm(0,1))
	//fmt.Printf("TM-Matrix (1,1): %+v\n", tm.GetElm(1,1))

}

func TestLineCopy(t *testing.T) {
	a1 := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	a2 := []float64{1, 2, 3}

	copy(a1[:], a2[:])
	fmt.Printf("%+v\n", a1)
	assert.EqualValues(t, []float64{1, 2, 3, 0, 0, 0, 0, 0, 0, 0}, a1)

	a3 := []float64{0, 0, 1, 2, 3, 0, 0, 0, 0, 0}
	a4 := []float64{0, 0, 0}

	copy(a4[:], a3[:])
	a3[0] = 1
	assert.EqualValues(t, []float64{0, 0, 1}, a4)
}
