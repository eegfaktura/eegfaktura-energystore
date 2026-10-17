package excel

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// isolatedTempDir points the process temp dir at an empty directory for the
// test, so the test can check that nothing is left behind.
func isolatedTempDir(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return dir
}

func requireEmptyDir(t *testing.T, dir string) {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// addForgedEntry adds an entry that declares more uncompressed bytes than it
// holds. excelize checks the declared sizes; archive/zip refuses to read more
// bytes than an entry declares.
func addForgedEntry(t *testing.T, zw *zip.Writer, name string, declared uint64) {
	data := []byte("<worksheet/>")
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               name,
		Method:             zip.Store,
		CompressedSize64:   uint64(len(data)),
		UncompressedSize64: declared,
	})
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
}

func requireUnzipLimitError(t *testing.T, err error) {
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "unzip size exceeds"), err.Error())
}

func TestOpenReaderRejectsOversizedEntry(t *testing.T) {
	tmp := isolatedTempDir(t)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addForgedEntry(t, zw, "xl/worksheets/sheet1.xml", uint64(openOptions.UnzipSizeLimit)+1)
	require.NoError(t, zw.Close())

	f, closeFile, err := OpenReader(bytes.NewReader(buf.Bytes()), "bomb.xlsx")
	requireUnzipLimitError(t, err)
	require.Nil(t, f)
	require.Nil(t, closeFile)
	requireEmptyDir(t, tmp)
}

// Each entry stays below the limit, their declared total does not. The first
// worksheet is above UnzipXMLSizeLimit, so excelize unzips it to a temp file
// before the second entry crosses the limit.
func TestOpenReaderRejectsOversizedTotalAndCleansUp(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("xl/worksheets/sheet1.xml")
	require.NoError(t, err)
	first := openOptions.UnzipXMLSizeLimit + 1<<20
	_, err = w.Write(bytes.Repeat([]byte(" "), int(first)))
	require.NoError(t, err)
	addForgedEntry(t, zw, "xl/worksheets/sheet2.xml", uint64(openOptions.UnzipSizeLimit-first)+1)
	require.NoError(t, zw.Close())

	// Without OpenReader's cleanup: excelize fails, returns no File and
	// leaves the temp file of the first worksheet behind.
	plainDir := t.TempDir()
	opts := openOptions
	opts.TmpDir = plainDir
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()), opts)
	requireUnzipLimitError(t, err)
	require.Nil(t, f)
	left, err := os.ReadDir(plainDir)
	require.NoError(t, err)
	require.Len(t, left, 1)
	require.True(t, strings.HasPrefix(left[0].Name(), "excelize-"), left[0].Name())

	// OpenReader removes it: nothing at all is left in the temp dir.
	tmp := isolatedTempDir(t)
	f, closeFile, err := OpenReader(bytes.NewReader(buf.Bytes()), "bomb.xlsx")
	requireUnzipLimitError(t, err)
	require.Nil(t, f)
	require.Nil(t, closeFile)
	requireEmptyDir(t, tmp)
}

// The largest real workbook in the repo (21.7 MB, 153 MB unzipped) still opens
// with the limits, through OpenReader (upload path) and OpenExceFile, and
// closing it leaves no temp files behind.
func TestOpenWorkbookWithinUnzipLimits(t *testing.T) {
	const path = "../test/zaehlpunkte-beispieldatei.xlsx"
	tmp := isolatedTempDir(t)

	r, err := os.Open(path)
	require.NoError(t, err)
	defer r.Close()
	f, closeFile, err := OpenReader(r, path)
	require.NoError(t, err)
	require.NotEmpty(t, f.GetSheetList())
	closeFile()
	requireEmptyDir(t, tmp)

	f, err = OpenExceFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, f.GetSheetList())
	require.NoError(t, f.Close())
	requireEmptyDir(t, tmp)
}
