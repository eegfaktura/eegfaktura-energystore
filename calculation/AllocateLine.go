package calculation

import (
	"at.ourproject/energystore/model"
)

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
