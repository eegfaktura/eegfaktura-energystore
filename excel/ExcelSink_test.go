package excel

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExportExcel(t *testing.T) {
	viper.Set("persistence.path", "../../../rawdata")
	err := ExportExcel("RC100181", 2023, 3)
	assert.NoError(t, err)
}
