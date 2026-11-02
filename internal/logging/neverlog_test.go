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
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The never-log test (logging concept point 4, dev#45 L11) reads the source of every package with
// the Go type checker. A log call fails the suite when it
//   - prints with %+v or %#v (a dump, whatever the value is);
//   - gets a value of a model or client type, with any verb: dumpPackages below;
//   - names a personal or secret value: sensitive below, in an argument or as the text in front of
//     a verb. Metering-point numbers are ids and may be logged (decided, W-261008-ca2).
//
// Errors, numbers and booleans carry no such name. A legitimate exception is an entry in
// neverLogAllowed with its reason; an entry that no longer matches fails the suite too.

const module = "at.ourproject/energystore"

var neverLogAllowed = []struct{ file, finding, reason string }{
	{"rest/restServer.go", "name Username",
		"audit.operator on the raw data deletion: decided exception (2026-10-30, W-261008-ca2), superuser-only tool; the module writes the field only on rawdata.delete.*"},
}

const sensitiveNames = `(?i)(first_?name|last_?name|e_?mail|iban|passw(or)?d|secret|token|user_?name|user_?id|subject)`

var (
	sensitive = regexp.MustCompile(sensitiveNames)
	// labelled is the name as the label of the value that follows: "password=%s".
	labelled = regexp.MustCompile(sensitiveNames + `\W{0,3}$`)
)

// dumpPackages hold the client types; the packages of this module hold the model types.
var dumpPackages = []string{
	"github.com/eclipse/paho.mqtt.golang", "net/http", "github.com/coreos/go-oidc/v3/oidc",
	"github.com/golang-jwt/jwt/v4", "google.golang.org/grpc",
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

// loadSource lists the module with `go list`; the export data of the build lets the type checker
// read every dependency without compiling it from source.
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
	s := &scanner{fset: source.fset, root: source.root, findings: map[string]bool{}}
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
	source.once.Do(loadSource)
	require.NoError(t, source.err)
	s := scanSource(t)
	require.Greater(t, s.calls, 150, "the scan must reach the log calls of the service")

	used := make([]bool, len(neverLogAllowed))
next:
	for f := range s.findings {
		file, rest, _ := strings.Cut(f, ":")
		_, finding, _ := strings.Cut(rest, ": ")
		for i, a := range neverLogAllowed {
			if a.file == file && a.finding == finding {
				used[i] = true
				continue next
			}
		}
		t.Errorf("never-log: %s (fix it, or add it to neverLogAllowed with a reason)", f)
	}
	for i, a := range neverLogAllowed {
		assert.NotEmpty(t, a.reason, a.file)
		assert.True(t, used[i], "stale neverLogAllowed entry: %s %s", a.file, a.finding)
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

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/model"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type person struct{ Firstname, LastName, IBAN, Password, Username, Subject string }

func bad(ctx context.Context, cl mqtt.Client, hc *http.Client, m *model.CounterPointMeta,
	ms []*model.CounterPointMeta, byId map[string]model.RawSourceMeta, s, email, value string, p person) {
	glog.Infof("lost (%+v)", s) // hit dump verb %+v
	glog.Infof("lost (%#v)", s) // hit dump verb %#v
	glog.Infof("lost %v", cl) // hit type mqtt.Client
	glog.Errorf("%s", hc) // hit type http.Client
	glog.V(5).Infof("meta %v", m) // hit type model.CounterPointMeta
	glog.WarningContextf(ctx, "meta %v", ms) // hit type model.CounterPointMeta
	glog.ErrorContext(ctx, byId) // hit type model.RawSourceMeta
	glog.Info(email) // hit name email
	glog.Infof("%s", p.Firstname) // hit name Firstname
	glog.Infof("%s", p.LastName) // hit name LastName
	glog.Infof("%s", p.IBAN) // hit name IBAN
	glog.Infof("%s", p.Password) // hit name Password
	glog.Infof("%s", p.Subject) // hit name Subject
	glog.Infof("password=%s", value) // hit label password=
	glog.Infof("mail: %s email: %s", s, value) // hit label email:
	log.Printf("%s", p.Username) // hit name Username
	glog.Error(fmt.Sprintf("x %+v", s)) // hit dump verb %+v
	logging.Log(ctx, 0, slog.LevelInfo, "x", slog.String("user.email", s)) // hit key user.email
	logging.Log(ctx, 0, slog.LevelInfo, "x", slog.Any("meta", m)) // hit type model.CounterPointMeta
	logging.AuditOperator(p.Username) // hit name Username
}

func good(ctx context.Context, tok mqtt.Token, vf *model.VfeegError, emailVerified bool, tokens []string, meteringPoint string, err error) {
	glog.Infof("metering point %s: %v", meteringPoint, err)
	glog.Infof("token check failed: %v", err)
	glog.Errorf("password rejected: %s", errors.New("x"))
	glog.Error(tok.Error())
	glog.Errorf("%v", vf)
	glog.Infof("%t %d", emailVerified, len(tokens))
	glog.Info("No User:Password in request!")
	fmt.Printf("not a log line %+v", tok)
}
`

func TestNeverLogScannerFindsPlantedLines(t *testing.T) {
	source.once.Do(loadSource)
	require.NoError(t, source.err)
	f, err := parser.ParseFile(source.fset, "fixture.go", neverLogFixture, parser.SkipObjectResolution)
	require.NoError(t, err)
	s := &scanner{fset: source.fset, root: source.root, findings: map[string]bool{}}
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
