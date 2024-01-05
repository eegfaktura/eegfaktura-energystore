package mocks

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/mock"
	"reflect"
)

func Copy(source interface{}, destin interface{}) {
	x := reflect.ValueOf(source)
	if x.Kind() == reflect.Ptr {
		starX := x.Elem()
		y := reflect.New(starX.Type())
		starY := y.Elem()
		starY.Set(starX)
		reflect.ValueOf(destin).Elem().Set(y.Elem())
	} else {
		destin = x.Interface()
	}
}

type MockBowStorage struct{ mock.Mock }
type MockBowRange struct {
	calls   int
	Entries []*model.RawSourceLine
	mock.Mock
}

func (ms *MockBowStorage) GetLine(line *model.RawSourceLine) error {
	_ = ms.Called(line)
	return nil
}

func (ms *MockBowStorage) SetLines(line []*model.RawSourceLine) error {
	args := ms.Called(line)
	return args.Error(0)
}

func (ms *MockBowStorage) SetMeta(line *model.RawSourceMeta) error {
	args := ms.Called(line)
	return args.Error(0)
}

func (ms *MockBowStorage) GetMeta(b string) (*model.RawSourceMeta, error) {
	args := ms.Called(b)
	return args.Get(0).(*model.RawSourceMeta), nil
}

func (ms *MockBowStorage) ListBuckets() ([]string, error) {
	_ = ms.Called()
	return []string{}, nil
}

func (ms *MockBowStorage) GetLineRange(bucket, key, until string) ebow.IRange {
	args := ms.Called(bucket, key, until)
	return args.Get(0).(ebow.IRange)
}

func (mr *MockBowRange) Next(result interface{}) bool {
	_ = mr.Called(result)
	if mr.calls >= len(mr.Entries) {
		return false
	}
	Copy(mr.Entries[mr.calls], result)
	mr.calls += 1
	return true
}

func (mr *MockBowRange) Close() {
	return
}

func (mr *MockBowRange) Err() error {
	args := mr.Called()
	return args.Error(0)
}
