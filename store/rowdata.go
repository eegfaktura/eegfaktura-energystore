package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"strings"
	"sync"
)

type ebowLogger struct {
	level glog.Level
}

func (el ebowLogger) Infof(format string, args ...interface{}) {
	glog.V(el.level).Infof(format, args...)
}

func (el ebowLogger) Warningf(format string, args ...interface{}) {
	glog.Warningf(format, args...)
}

func (el ebowLogger) Errorf(format string, args ...interface{}) {
	glog.Errorf(format, args...)
}

func (el ebowLogger) Debugf(format string, args ...interface{}) {
	glog.V(el.level).Infof(format, args...)
}

// Prevent multiple goroutines from accessing the same resource at the same time (forces turn taking)
type Turns struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func newTurns() *Turns {
	t := &Turns{
		m: make(map[string]*sync.Mutex),
	}
	return t
}

// Lock a resource by given name
func (t *Turns) lock(name string) func() {
	t.mu.Lock()
	l, ok := t.m[name]
	if !ok {
		l = &sync.Mutex{}
		t.m[name] = l
	}
	t.mu.Unlock()

	l.Lock()
	return l.Unlock
}

type BowStorage struct {
	db     *ebow.DB
	unlock func()
}

var turns = newTurns()

func OpenStorage(tenant string) (*BowStorage, error) {
	t := strings.ToLower(tenant)
	basePath := viper.GetString("persistence.path")
	unlock := turns.lock(t)
	db, err := ebow.Open(fmt.Sprintf("%s/%s", basePath, t), ebow.SetLogger(ebowLogger{5}))
	if err != nil {
		unlock()
		return nil, err
	}
	return &BowStorage{db, unlock}, nil
}

func (b *BowStorage) Close() {
	_ = b.db.Close()
	b.unlock()
	return
}

func (b *BowStorage) SetLines(line []*model.RawSourceLine) error {
	return b.SetLinesRaw("rawdata", line)
}

func (b *BowStorage) SetLinesG2(line []*model.RawSourceLine) error {
	return b.SetLinesRaw("rawdata", line)
}

func (b *BowStorage) SetLinesG3(line []*model.RawSourceLine) error {
	return b.SetLinesRaw("rawdata", line)
}

func (b *BowStorage) SetLinesRaw(bucket string, line []*model.RawSourceLine) error {
	i := make([]interface{}, len(line))
	for l := range line {
		i[l] = line[l]
	}
	return b.db.Bucket(bucket).PutBatch(i)
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

func (b *BowStorage) GetLineRange(bucket, key, until string) *ebow.Range {
	return b.db.Bucket("rawdata").Range(fmt.Sprintf("%s/%s", bucket, key), fmt.Sprintf("%s/%s", bucket, until))
}

func (b *BowStorage) GetLine(line *model.RawSourceLine) error {
	return b.db.Bucket("rawdata").Get(line.Id, line)
}
func (b *BowStorage) GetLineG2(line *model.RawSourceLine) error {
	return b.db.Bucket("rawdata").Get(line.Id, line)
}
func (b *BowStorage) GetLineG3(line *model.RawSourceLine) error {
	return b.db.Bucket("rawdata").Get(line.Id, line)
}

func GenerateCPKey(year int, month int) string {
	return fmt.Sprintf("CP/%.4d/%.2d", year, month)
}
