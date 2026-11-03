package logging

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// neverLogWords is the list of all services (dev#45). An identifier is split into words (camelCase,
// snake_case) and a run of up to three words, joined, is compared with an entry: "FirstName",
// "first_name" and "firstname" are "firstname", "tokens" and "subscription" are not on the list.
var neverLogWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`password passwd pwd secret token jwt bearer authorization credential
		creds cookie apikey iban bic firstname lastname vorname nachname name1 name2 street phone email
		mailaddress recipient sentto owner accountholder username userid sub subject payload body`) {
		neverLogWords[w] = true
	}
}

// words splits an identifier: "basicAuthHeader" → basic auth header, "HTTPServer" → http server,
// "user_id" → user id, "Name1" → name1.
func words(id string) []string {
	var ws []string
	rs := []rune(id)
	start := 0
	cut := func(end, next int) {
		if end > start {
			ws = append(ws, strings.ToLower(string(rs[start:end])))
		}
		start = next
	}
	for i, r := range rs {
		switch {
		case r == '_' || r == '-' || r == '.':
			cut(i, i+1)
		case i > start && unicode.IsUpper(r):
			prev := rs[i-1]
			endOfAcronym := unicode.IsUpper(prev) && i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || endOfAcronym {
				cut(i, i)
			}
		}
	}
	cut(len(rs), len(rs))
	return ws
}

// sensitiveName reports whether an identifier names a value of the list.
func sensitiveName(id string) bool {
	ws := words(id)
	for i := range ws {
		joined := ""
		for j := i; j < len(ws) && j < i+3; j++ {
			joined += ws[j]
			if neverLogWords[joined] {
				return true
			}
		}
	}
	return false
}

var verb = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)

// isLogCall: the calls that write a line or build a field of one.
func isLogCall(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}
	has := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(fn.Name(), p) })
	}
	switch fn.Pkg().Path() {
	case module + "/internal/logging/glog":
		return has("Info", "Warning", "Error", "Fatal", "Exit")
	case module + "/internal/logging":
		return fn.Name() == "Log" || fn.Name() == "Event" || fn.Name() == "AuditOperator"
	case "log/slog": // the functions and Logger methods that write or build fields, not the handler side
		if recv := fn.Signature().Recv(); recv != nil {
			return strings.HasSuffix(types.TypeString(recv.Type(), nil), "slog.Logger")
		}
		return !strings.HasPrefix(fn.Name(), "New") && !strings.HasPrefix(fn.Name(), "Set")
	case "log":
		return has("Print", "Fatal", "Panic")
	case "fmt": // stdout ends up in the container log as well (as in the backend)
		return fn.Name() == "Print" || fn.Name() == "Println" || fn.Name() == "Printf"
	}
	return false
}

func calledFunc(call *ast.CallExpr, info *types.Info) *types.Func {
	var id *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	}
	if id == nil {
		return nil
	}
	fn, _ := info.Uses[id].(*types.Func)
	return fn
}

// clientPackages hold client interfaces whose dynamic value prints its options.
var clientPackages = []string{
	"github.com/eclipse/paho.mqtt.golang", "net/http", "github.com/coreos/go-oidc/v3/oidc",
	"github.com/golang-jwt/jwt/v4", "google.golang.org/grpc",
}

func isClientPackage(path string) bool {
	return path == module || strings.HasPrefix(path, module+"/") || slices.Contains(clientPackages, path)
}

// printsItself: an error or a fmt.Stringer prints what it chooses, not its fields. Only the method
// set of t counts (addressable false): fmt does not call a pointer-receiver String on a value.
func printsItself(t types.Type) bool {
	for _, name := range []string{"Error", "String"} {
		obj, _, _ := types.LookupFieldOrMethod(t, false, nil, name)
		if f, ok := obj.(*types.Func); ok {
			sig := f.Signature()
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
				return true
			}
		}
	}
	return false
}

// printedWhole is the type inside t that a line would print field by field: any struct (as in the
// backend), http.Header, or a client interface of this module or clientPackages. "" if none.
func printedWhole(t types.Type) string {
	if t == nil || printsItself(t) {
		return ""
	}
	switch t := types.Unalias(t).(type) {
	case *types.Pointer:
		return printedWhole(t.Elem())
	case *types.Slice:
		return printedWhole(t.Elem())
	case *types.Array:
		return printedWhole(t.Elem())
	case *types.Map:
		return cmp.Or(printedWhole(t.Key()), printedWhole(t.Elem()))
	case *types.Struct:
		return "struct"
	case *types.Named:
		pkg := t.Obj().Pkg()
		name := t.Obj().Name()
		if pkg == nil {
			return ""
		}
		name = pkg.Name() + "." + name
		if pkg.Path() == "net/http" && t.Obj().Name() == "Header" {
			return name // holds Authorization and cookies
		}
		switch u := t.Underlying().(type) {
		case *types.Struct:
			return name
		case *types.Interface:
			if isClientPackage(pkg.Path()) {
				return name
			}
			return ""
		default:
			return printedWhole(u)
		}
	}
	return ""
}

// carriesText: an error, a number or a boolean holds no personal value by its name.
func carriesText(t types.Type) bool {
	if b, ok := t.Underlying().(*types.Basic); ok && b.Info()&(types.IsNumeric|types.IsBoolean) != 0 {
		return false
	}
	return !types.Implements(t, errorType) && !types.Implements(types.NewPointer(t), errorType)
}

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// scanner collects the findings of the scanned packages.
type scanner struct {
	fset     *token.FileSet
	info     *types.Info
	root     string
	findings map[string]bool // "file:line: finding"
	used     map[string]bool // the neverLogAllowed keys that matched
}

func newScanner() *scanner {
	return &scanner{fset: source.fset, root: source.root, findings: map[string]bool{}, used: map[string]bool{}}
}

func (s *scanner) report(pos token.Pos, finding string) {
	p := s.fset.Position(pos)
	file := p.Filename
	if rel, err := filepath.Rel(s.root, file); err == nil {
		file = filepath.ToSlash(rel)
	}
	if key := file + ": " + finding; neverLogAllowed[key] != "" {
		s.used[key] = true
		return
	}
	s.findings[fmt.Sprintf("%s:%d: %s", file, p.Line, finding)] = true
}

func (s *scanner) file(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn := calledFunc(call, s.info); fn != nil && isLogCall(fn) {
				s.logCall(fn, call)
			}
		}
		return true
	})
}

func (s *scanner) logCall(fn *types.Func, call *ast.CallExpr) {
	s.args(fn, call.Args)
	for _, arg := range call.Args { // a message built in place
		ast.Inspect(arg, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if f := calledFunc(c, s.info); f != nil && f.Pkg() != nil && f.Pkg().Path() == "fmt" && !isLogCall(f) {
					s.args(f, c.Args)
				}
			}
			return true
		})
	}
}

// args: the format text is read for dump verbs only (string literals are not scanned for names);
// every other argument for what it prints.
func (s *scanner) args(fn *types.Func, args []ast.Expr) {
	format := strings.HasSuffix(fn.Name(), "f")
	for _, arg := range args {
		tv := s.info.Types[arg]
		if tv.Value != nil && tv.Value.Kind() == constant.String {
			if format {
				format = false
				for _, v := range verb.FindAllString(constant.StringVal(tv.Value), -1) {
					if strings.HasSuffix(v, "v") && strings.ContainsAny(v, "+#") {
						s.report(arg.Pos(), "dump verb "+v)
					}
				}
			}
			continue
		}
		s.value(arg, tv.Type)
	}
}

func (s *scanner) value(arg ast.Expr, t types.Type) {
	if t == nil {
		return
	}
	if w := printedWhole(t); w != "" {
		s.report(arg.Pos(), "type "+w)
	}
	if !carriesText(t) {
		return
	}
	ast.Inspect(arg, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if fn := calledFunc(c, s.info); fn != nil && isLogCall(fn) {
				return false // scanned as a log call of its own
			}
		}
		if id, ok := n.(*ast.Ident); ok && sensitiveName(id.Name) {
			if _, pkg := s.info.Uses[id].(*types.PkgName); !pkg {
				s.report(id.Pos(), "name "+id.Name)
			}
		}
		return true
	})
}
