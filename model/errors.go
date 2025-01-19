package model

import "net/http"

type VfeegError struct {
	Code     int
	HttpCode int
	Tag      string
	Err      error
}

func (r *VfeegError) Error() string {
	return r.Err.Error()
}

func Wrap(err error, code, httpCode int, tag string) *VfeegError {
	return &VfeegError{
		Code:     code,
		HttpCode: httpCode,
		Tag:      tag,
		Err:      err,
	}
}

func PartialWrap(code, httpCode int, tag string) func(err error) *VfeegError {
	return func(err error) *VfeegError {
		return &VfeegError{
			Code:     code,
			HttpCode: httpCode,
			Tag:      tag,
			Err:      err,
		}
	}
}

var ErrParseJson = PartialWrap(2000, http.StatusBadRequest, "parse_json")

var ErrConnectDatabase = PartialWrap(999, http.StatusBadRequest, "connect_database")
var ErrOpenTx = PartialWrap(998, http.StatusBadRequest, "open_transaction")

var ErrNoEntries = PartialWrap(1000, http.StatusNotFound, "no_entries")
