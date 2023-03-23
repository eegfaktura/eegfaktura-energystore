package excel

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/store"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/golang/glog"
)

func ImportFile(tenant, filename, sheet string, file io.Reader) error {
	fmt.Printf("Start Import\n")
	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	return ImportEEG(db, file, filename, sheet, tenant)
}

func ImportEEG(db *store.BowStorage, r io.Reader, filename, sheet, tenant string) error {
	/*
		Path:  "/home/petero/Downloads/AT00300DUMMY_20210101_20211231_202202181342.xlsx"
		Sheet: "ConsumptionDataReport"
	*/
	ext := filepath.Ext(filename)
	fmt.Printf("Import Raw-Data: %+v Ext: %s\n", filename, ext)
	if ext == ".xlsx" || ext == ".xls" {
		if f, err := OpenReader(r, filename); err == nil {
			defer f.Close()
			var err error
			var yearSet []int
			if yearSet, err = ImportExcelEnergyFile(f, sheet, db); err != nil {
				glog.Infof("Import Execl Error: %v\n", err)
				return err
			}
			for _, k := range yearSet {
				err = calculation.CalculateMonthlyDash(db, fmt.Sprintf("%d", k), calculation.CalculateEEG)
				if err != nil {
					return err
				}
			}
			return nil
		}
	}
	return errors.New("Invalid Text File")
}
