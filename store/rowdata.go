package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"fmt"
	"github.com/spf13/viper"
)

type BowStorage struct {
	db *ebow.DB
}

func OpenStorage(tenant string) (*BowStorage, error) {
	basePath := viper.GetString("persistence.path")
	db, err := ebow.Open(fmt.Sprintf("%s/%s", basePath, tenant))
	if err != nil {
		return nil, err
	}
	return &BowStorage{db}, nil
}

func (b *BowStorage) Close() error {
	return b.db.Close()
}

func (b *BowStorage) SetLines(line []*model.RawSourceLine) error {
	i := make([]interface{}, len(line))
	for l := range line {
		i[l] = line[l]
	}
	return b.db.Bucket("rawdata").PutBatch(i)
}

func (b *BowStorage) SetLine(line *model.RawSourceLine) error {
	return b.db.Bucket("rawdata").Put(line)
}

func (b *BowStorage) SetReport(line *model.EnergyReport) error {
	return b.db.Bucket("rawdata").Put(line)
}

func (b *BowStorage) GetReport(period string) (*model.EnergyReport, error) {
	var report model.EnergyReport = model.EnergyReport{}
	err := b.db.Bucket("rawdata").Get(period, &report)
	return &report, err
}

func (b *BowStorage) SetMeta(line *model.RawSourceMeta) error {

	return b.db.Bucket("metadata").Put(line)
}

func (b *BowStorage) GetMeta(key string) (*model.RawSourceMeta, error) {
	var rawMeta model.RawSourceMeta
	err := b.db.Bucket("metadata").Get(key, &rawMeta)
	return &rawMeta, err
}

func (b *BowStorage) GetLinePrefix(key string) *ebow.Iter {
	return b.db.Bucket("rawdata").Prefix(key)
}

func (b *BowStorage) GetLine(line *model.RawSourceLine) error {
	return b.db.Bucket("rawdata").Get(line.Id, line)
}
