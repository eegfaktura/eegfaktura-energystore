package excel

import (
	"at.ourproject/energystore/model"
	"context"
	"github.com/xuri/excelize/v2"
)

type EnergyRunner struct {
}

func ExportEnergyToExcel() error {
	return nil
}

type EnergySheet struct {
	name       string
	excel      *excelize.File
	handleLine chan *model.RawSourceLine
}

func (es *EnergySheet) run(ctx context.Context) {
	for {
		select {
		case line := <-es.handleLine:
			es.addLine(line)
		case <-ctx.Done():
			return
		}
	}
}

func (es *EnergySheet) addLine(line *model.RawSourceLine) {
}
