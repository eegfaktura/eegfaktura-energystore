package calculation

import (
	"math"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
)

func AllocDynamic1(line *model.RawSourceLine) *model.Matrix {

	lenConsumers := int(math.Max(float64(len(line.Consumers)), 1))
	lenProducers := int(math.Max(float64(len(line.Producers)), 1))

	resultArray := make([]float64, lenConsumers*lenProducers)
	lineResult := model.MakeMatrix(resultArray, lenConsumers, lenProducers)

	consumerSum := utils.Sum(line.Consumers)
	producerSum := utils.Sum(line.Producers)

	var alloc_prod_to_cons_factor = float64(0)
	if producerSum > float64(0) && consumerSum > float64(0) {
		alloc_prod_to_cons_factor = consumerSum / producerSum
	}

	for i, l := range line.Consumers {
		greenValue := float64(0)
		if alloc_prod_to_cons_factor > float64(0) {
			greenValue = l / alloc_prod_to_cons_factor
		}
		for j, pl := range line.Producers {
			var prod_factor = float64(0)
			if producerSum > float64(0) {
				prod_factor = pl / producerSum
			}
			lineResult.SetElm(i, j, math.Min(float64(l), float64(greenValue*prod_factor)))
		}
	}
	return model.Multiply(lineResult, model.NewUniformMatrix(lineResult.Cols, 1))
}

func AllocDynamicV2(consumerMatrix, producerMatrix *model.Matrix) (*model.Matrix, *model.Matrix, *model.Matrix) {

	// set identity matrix to filter allocated value
	consumerUnitMatix := model.MakeMatrix(make([]float64, 3), 3, 1)
	consumerUnitMatix.SetElm(2, 0, 1)
	allocResult := model.Multiply(consumerMatrix, consumerUnitMatix)

	// set identity matrix to filter shared value
	consumerUnitMatix.SetElm(2, 0, 0)
	consumerUnitMatix.SetElm(1, 0, 1)
	shareResult := model.Multiply(consumerMatrix, consumerUnitMatix)

	// set identity matrix to filter total produced value
	producerUnitMatix := model.MakeMatrix(make([]float64, 2), 2, 1)
	producerUnitMatix.SetElm(1, 0, 1)
	prodResult := model.Multiply(producerMatrix, producerUnitMatix)

	return allocResult, shareResult, prodResult
}
