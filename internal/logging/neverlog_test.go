package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The never-log test (logging concept point 4, dev#45 L11, Q2) reads the source of every package
// with the Go type checker. A log call (glog, internal/logging, log/slog, log, fmt.Print*) fails the
// suite when it
//   - prints with %+v or %#v (a dump, whatever the value is);
//   - gets a value that prints field by field, with any verb: any struct (a model above all),
//     http.Header, or a client interface (printedWhole). Errors and Stringers print what they choose;
//   - names a personal or secret value: an identifier with a word of neverLogWords. String literals
//     are not scanned. Metering-point numbers are ids and may be logged (decided, W-261008-ca2).
//
// Errors, numbers and booleans carry no such name. A legitimate exception is an entry in
// neverLogAllowed, keyed by file and finding (no line), with its reason; an entry that no longer
// matches fails the suite too. A false positive is fixed in the scanner, not allowed here.

const module = "at.ourproject/energystore"

var neverLogAllowed = map[string]string{
	"rest/restServer.go: name Username": "audit.operator on the raw data deletion: decided exception (2026-10-30, " +
		"W-261008-ca2), superuser-only tool; the module writes the field only on rawdata.delete.*",
}

type listedPackage struct {
	ImportPath, Dir, Export string
	GoFiles                 []string
}

var source struct {
	once     sync.Once
	root     string
	packages []listedPackage
	imp      types.Importer
	fset     *token.FileSet
	err      error
}

// loadedSource lists the module with `go list` once per test binary; the export data of the build
// lets the type checker read every dependency without compiling it from source. Skipped with -short.
func loadedSource(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("go list and the type check of the module: not with -short")
	}
	source.once.Do(loadSource)
	require.NoError(t, source.err)
}

func loadSource() {
	source.fset = token.NewFileSet()
	source.root, source.err = filepath.Abs("../..")
	if source.err != nil {
		return
	}
	cmd := exec.Command("go", "list", "-e", "-deps", "-export", "-json=ImportPath,Dir,Export,GoFiles", "./...")
	cmd.Dir = source.root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		source.err = fmt.Errorf("go list: %w", err)
		return
	}
	exports := map[string]string{}
	for dec := json.NewDecoder(bytes.NewReader(out)); dec.More(); {
		var p listedPackage
		if source.err = dec.Decode(&p); source.err != nil {
			return
		}
		exports[p.ImportPath] = p.Export
		if p.ImportPath == module || strings.HasPrefix(p.ImportPath, module+"/") {
			source.packages = append(source.packages, p)
		}
	}
	source.imp = importer.ForCompiler(source.fset, "gc", func(path string) (io.ReadCloser, error) {
		if exports[path] == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(exports[path])
	})
}

func newInfo() *types.Info {
	return &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}}
}

// scanFiles type-checks files as one package and scans those that are not generated.
func scanFiles(t *testing.T, path string, files []*ast.File, s *scanner) {
	t.Helper()
	conf := types.Config{Importer: source.imp, Error: func(err error) { t.Errorf("%s: %v", path, err) }}
	s.info = newInfo()
	_, _ = conf.Check(path, source.fset, files, s.info)
	for _, f := range files {
		if !ast.IsGenerated(f) {
			s.file(f)
		}
	}
}

func scanSource(t *testing.T) *scanner {
	t.Helper()
	s := newScanner()
	for _, p := range source.packages {
		var files []*ast.File
		for _, name := range p.GoFiles {
			f, err := parser.ParseFile(source.fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
			require.NoError(t, err)
			files = append(files, f)
		}
		if p.ImportPath != module {
			scanFiles(t, p.ImportPath, files, s)
			continue
		}
		for _, f := range files { // the root holds three func main: each file is its own program
			scanFiles(t, p.ImportPath, []*ast.File{f}, s)
		}
	}
	return s
}

func TestSourceNeverLogs(t *testing.T) {
	loadedSource(t)
	// that the scan finds what it should is TestNeverLogScannerFindsPlantedLines
	require.NotEmpty(t, source.packages, "go list found the packages of the module")
	s := scanSource(t)

	for f := range s.findings {
		t.Errorf("never-log: %s (fix it, or add it to neverLogAllowed with a reason)", f)
	}
	for key := range neverLogAllowed {
		assert.True(t, s.used[key], "stale neverLogAllowed entry: %s", key)
	}
}

// neverLogFixture is the scanner's own test: each line marked "// hit <finding>" must be found with
// exactly that finding, every other line must pass.
const neverLogFixture = `package fixture

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/model"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type person struct{ Firstname, LastName, IBAN, Password, Username, Subject string }

func bad(ctx context.Context, cl mqtt.Client, hc *http.Client, m *model.CounterPointMeta,
	ms []*model.CounterPointMeta, byId map[string]model.RawSourceMeta, s, email, value string, p person,
	hdr http.Header, authorization, bearer, creds, apiKey, api_key, payload, body, name1, sentTo, jwtToken string,
	tok mqtt.Token) {
	glog.Infof("lost (%+v)", s) // hit dump verb %+v
	glog.Infof("lost (%#v)", s) // hit dump verb %#v
	glog.Infof("lost %v", cl) // hit type mqtt.Client
	glog.Errorf("%s", hc) // hit type http.Client
	glog.V(5).Infof("meta %v", m) // hit type model.CounterPointMeta
	glog.WarningContextf(ctx, "meta %v", ms) // hit type model.CounterPointMeta
	glog.ErrorContext(ctx, byId) // hit type model.RawSourceMeta
	glog.Infof("%v", p.Subject == "") // no name in a boolean
	glog.Info(email) // hit name email
	glog.Infof("%s", p.Firstname) // hit name Firstname
	glog.Infof("%s", p.LastName) // hit name LastName
	glog.Infof("%s", p.IBAN) // hit name IBAN
	glog.Infof("%s", p.Password) // hit name Password
	glog.Infof("%s", p.Subject) // hit name Subject
	log.Printf("%s", p.Username) // hit name Username
	glog.Error(fmt.Sprintf("x %+v", s)) // hit dump verb %+v
	logging.Log(ctx, 0, slog.LevelInfo, "x", slog.Any("meta", m)) // hit type model.CounterPointMeta
	logging.AuditOperator(p.Username) // hit name Username
	glog.Infof("%v", hdr) // hit type http.Header
	glog.Info(authorization) // hit name authorization
	glog.Info(bearer) // hit name bearer
	glog.Info(creds) // hit name creds
	glog.Info(apiKey) // hit name apiKey
	glog.Info(api_key) // hit name api_key
	glog.Info(payload) // hit name payload
	glog.Info(body) // hit name body
	glog.Info(name1) // hit name name1
	glog.Info(sentTo) // hit name sentTo
	glog.Info(jwtToken) // hit name jwtToken
	glog.Infof("%v", tok) // hit type mqtt.Token
}

func whole(p person, local struct{ A int }, m *model.CounterPointMeta, email string) {
	glog.Infof("%v", local) // hit type struct
	glog.Infof("%d", []person{p}) // hit type fixture.person
	fmt.Printf("%v", m) // hit type model.CounterPointMeta
	fmt.Println(p) // hit type fixture.person
	fmt.Print(email) // hit name email
}

func good(ctx context.Context, tok mqtt.Token, vf *model.VfeegError, emailVerified bool, tokens []string,
	meteringPoint, subscription, nameServer, tokenizer, value string, err error, t time.Time) {
	glog.Infof("metering point %s: %v", meteringPoint, err)
	glog.Infof("token check failed: %v", err)
	glog.Errorf("password rejected: %s", errors.New("x"))
	glog.Infof("password=%s email: %s", value, value) // literals are not scanned
	logging.Log(ctx, 0, slog.LevelInfo, "x", slog.String("user.email", value))
	glog.Error(tok.Error())
	glog.Errorf("%v", vf)
	glog.Infof("%t %d", emailVerified, len(tokens))
	glog.Info(subscription, nameServer, tokenizer) // not a whole word of the list
	glog.Infof("%v", t) // a Stringer
	glog.Info("No User:Password in request!")
	_ = fmt.Sprintf("not a log line %+v", tok)
}
`

func TestNeverLogScannerFindsPlantedLines(t *testing.T) {
	loadedSource(t)
	f, err := parser.ParseFile(source.fset, "fixture.go", neverLogFixture, parser.SkipObjectResolution)
	require.NoError(t, err)
	s := newScanner()
	scanFiles(t, "fixture", []*ast.File{f}, s)

	want := []string{}
	for i, line := range strings.Split(neverLogFixture, "\n") {
		if _, hit, ok := strings.Cut(line, "// hit "); ok {
			want = append(want, fmt.Sprintf("fixture.go:%d: %s", i+1, hit))
		}
	}
	got := []string{}
	for k := range s.findings {
		got = append(got, k)
	}
	assert.ElementsMatch(t, want, got)
}

func TestNeverLogWords(t *testing.T) {
	for id, want := range map[string][]string{
		"basicAuthHeader": {"basic", "auth", "header"}, "HTTPServer": {"http", "server"}, "user_id": {"user", "id"},
		"Name1": {"name1"}, "IBAN": {"iban"}, "meteringPoint": {"metering", "point"}, "x": {"x"},
	} {
		assert.Equal(t, want, words(id), id)
	}
	for _, id := range []string{"FirstName", "first_name", "firstname", "userID", "UserId", "mailAddress", "AccountHolder", "Sub"} {
		assert.True(t, sensitiveName(id), id)
	}
	for _, id := range []string{"tokens", "subscription", "ownership", "nameServer", "Submit", "bodyless", "id"} {
		assert.False(t, sensitiveName(id), id)
	}
}
