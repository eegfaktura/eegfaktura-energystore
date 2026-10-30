package excel

import (
	"at.ourproject/energystore/store/ebow"
	"errors"
	"io"
	"path/filepath"

	"at.ourproject/energystore/internal/logging/glog"
)

func ImportFile(tenant, ecId, filename, sheet string, file io.Reader) error {
	glog.Info("Start Import")
	db, err := ebow.OpenStorage(tenant, ecId)
	if err != nil {
		return err
	}
	defer func() { db.Close() }()

	return ImportEEG(db, file, filename, sheet, tenant)
}

func ImportEEG(db *ebow.BowStorage, r io.Reader, filename, sheet, tenant string) error {
	/*
		Path:  "/home/petero/Downloads/AT00300DUMMY_20210101_20211231_202202181342.xlsx"
		Sheet: "ConsumptionDataReport"
	*/
	ext := filepath.Ext(filename)
	glog.Infof("Import Raw-Data: %+v Ext: %s", filename, ext)
	if ext == ".xlsx" || ext == ".xls" {
		f, closeFile, err := OpenReader(r, filename)
		if err != nil {
			glog.Infof("Open Excel Error: %v", err)
			return errors.New("Invalid Text File")
		}
		defer closeFile()
		unlock := db.LockWrites() // the MQTT import writes cpmeta/0 and the rows of the same store (#33)
		defer unlock()
		if err = ImportExcelEnergyFileNew(f, sheet, db); err != nil {
			glog.Infof("Import Execl Error: %v\n", err)
			return err
		}
		return nil
	}
	return errors.New("Invalid Text File")
}
