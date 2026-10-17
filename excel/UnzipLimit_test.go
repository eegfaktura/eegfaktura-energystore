package excel

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A workbook whose worksheet declares an uncompressed size above the limit is
// rejected before anything is unzipped. archive/zip refuses to read more bytes
// than an entry declares, so the declared size is binding.
func TestOpenReaderRejectsOversizedWorkbook(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	data := []byte("<worksheet/>")
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               "xl/worksheets/sheet1.xml",
		Method:             zip.Store,
		CompressedSize64:   uint64(len(data)),
		UncompressedSize64: uint64(unzipSizeLimit) + 1,
	})
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	f, err := OpenReader(bytes.NewReader(buf.Bytes()), "bomb.xlsx")
	require.Error(t, err)
	require.Nil(t, f)
	require.True(t, strings.Contains(err.Error(), "unzip size exceeds"), err.Error())
}

// The largest real workbook in the repo (21.7 MB, 153 MB unzipped) still opens
// with the limits, through OpenReader (upload path) and OpenExceFile.
func TestOpenWorkbookWithinUnzipLimits(t *testing.T) {
	const path = "../test/zaehlpunkte-beispieldatei.xlsx"

	r, err := os.Open(path)
	require.NoError(t, err)
	defer r.Close()
	f, err := OpenReader(r, path)
	require.NoError(t, err)
	require.NotEmpty(t, f.GetSheetList())
	require.NoError(t, f.Close())

	f, err = OpenExceFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, f.GetSheetList())
	require.NoError(t, f.Close())
}
