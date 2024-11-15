package calculation

import (
	"fmt"

	"github.com/eegfaktura/eegfaktura-energystore/model"
	"github.com/eegfaktura/eegfaktura-energystore/store"
)

func CalcHourSum(db *store.BowStorage, period string) ([]*model.Matrix, []*model.Matrix) {

	consHours := []*model.Matrix{}
	prodHours := []*model.Matrix{}

	hours := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}

	for i := 0; i < len(hours); i++ {
		iter := db.GetLinePrefix(fmt.Sprintf("CP/%s/%d", period, hours[i]))

		var rCons *model.Matrix
		var rProd *model.Matrix

		var _line model.RawSourceLine
		for iter.Next(&_line) {
			line := _line.Copy(len(_line.Consumers))

			if rCons == nil {
				rCons = model.MakeMatrix(line.Consumers, len(line.Consumers), 1)
			} else {
				rCons.Add(model.MakeMatrix(line.Consumers, len(line.Consumers), 1))
			}
			if rProd == nil {
				rProd = model.MakeMatrix(line.Producers, len(line.Producers), 1)
			} else {
				rProd.Add(model.MakeMatrix(line.Producers, len(line.Producers), 1))
			}
		}

		consHours = append(consHours, rCons)
		prodHours = append(prodHours, rProd)
	}

	return consHours, prodHours
}
