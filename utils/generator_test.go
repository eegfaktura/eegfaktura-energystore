package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMessageId(t *testing.T) {
	id := NewMessageId("RC100130")
	// community + yyyyMMddHHmmss + milliseconds (3) + random (10 digits)
	assert.Regexp(t, regexp.MustCompile(`^RC100130\d{4}(0[1-9]|1[0-2])\d{2}\d{6}\d{3}\d{10}$`), id)
	assert.NotEqual(t, id, NewMessageId("RC100130"), "random part")
}
