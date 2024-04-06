package excel

import (
	"at.ourproject/energystore/store"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/golang/glog"
)

func ImportFile(tenant, ecId, filename, sheet string, file io.Reader) error {
	fmt.Printf("Start Import\n")
	db, err := store.OpenStorage(tenant, ecId)
	if err != nil {
		return err
	}
	defer func() { db.Close() }()

	return ImportEEG(db, file, filename, sheet, tenant)
}

func ImportEEG(db *store.BowStorage, r io.Reader, filename, sheet, tenant string) error {
	/*
		Path:  "/home/petero/Downloads/AT00300DUMMY_20210101_20211231_202202181342.xlsx"
		Sheet: "ConsumptionDataReport"
	*/
	ext := filepath.Ext(filename)
	glog.Infof("Import Raw-Data: %+v Ext: %s", filename, ext)
	if ext == ".xlsx" || ext == ".xls" {
		if f, err := OpenReader(r, filename); err == nil {
			defer f.Close()
			var err error
			if err = ImportExcelEnergyFileNew(f, sheet, db); err != nil {
				glog.Infof("Import Execl Error: %v\n", err)
				return err
			}
			return nil
		}
	}
	return errors.New("Invalid Text File")
}
