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
)

var verb = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

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
	case "log/slog":
		return fn.Name() != "New" && fn.Name() != "SetDefault"
	case "log":
		return has("Print", "Fatal", "Panic")
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

func isDumpPackage(path string) bool {
	return path == module || strings.HasPrefix(path, module+"/") || slices.Contains(dumpPackages, path)
}

func implementsError(t types.Type) bool {
	return types.Implements(t, errorType) || types.Implements(types.NewPointer(t), errorType)
}

// dumpType is the model or client type inside t, "" if there is none.
func dumpType(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Pointer:
		return dumpType(t.Elem())
	case *types.Slice:
		return dumpType(t.Elem())
	case *types.Array:
		return dumpType(t.Elem())
	case *types.Map:
		return cmp.Or(dumpType(t.Key()), dumpType(t.Elem()))
	case *types.Named:
		pkg := t.Obj().Pkg()
		if pkg == nil || !isDumpPackage(pkg.Path()) || implementsError(t) {
			return ""
		}
		switch u := t.Underlying().(type) {
		case *types.Struct, *types.Interface:
			return pkg.Name() + "." + t.Obj().Name()
		default:
			return dumpType(u)
		}
	}
	return ""
}

// carriesText: an error, a number or a boolean holds no personal value by its name.
func carriesText(t types.Type) bool {
	if b, ok := t.Underlying().(*types.Basic); ok && b.Info()&(types.IsNumeric|types.IsBoolean) != 0 {
		return false
	}
	return !implementsError(t)
}

// scanner collects the findings of one package.
type scanner struct {
	fset     *token.FileSet
	info     *types.Info
	root     string
	calls    int
	findings map[string]bool // "file:line: finding"
}

func (s *scanner) report(pos token.Pos, finding string) {
	p := s.fset.Position(pos)
	file := p.Filename
	if rel, err := filepath.Rel(s.root, file); err == nil {
		file = filepath.ToSlash(rel)
	}
	s.findings[fmt.Sprintf("%s:%d: %s", file, p.Line, finding)] = true
}

func (s *scanner) file(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn := calledFunc(call, s.info); fn != nil && isLogCall(fn) {
				s.calls++
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
				if f := calledFunc(c, s.info); f != nil && f.Pkg() != nil && f.Pkg().Path() == "fmt" {
					s.args(f, c.Args)
				}
			}
			return true
		})
	}
}

func (s *scanner) args(fn *types.Func, args []ast.Expr) {
	format := -1
	for i, arg := range args {
		tv := s.info.Types[arg]
		if tv.Value == nil || tv.Value.Kind() != constant.String {
			s.value(arg, tv.Type)
			continue
		}
		text := constant.StringVal(tv.Value)
		if format < 0 && strings.HasSuffix(fn.Name(), "f") {
			format = i
			s.format(arg, text, args[i+1:])
		} else if fn.Pkg().Path() == "log/slog" && sensitive.MatchString(text) {
			s.report(arg.Pos(), "key "+text)
		}
	}
}

func (s *scanner) format(lit ast.Expr, text string, values []ast.Expr) {
	n := 0
	last := 0
	for _, m := range verb.FindAllStringIndex(text, -1) {
		v := text[m[0]:m[1]]
		before := text[last:m[0]]
		last = m[1]
		if strings.HasSuffix(v, "%") {
			continue
		}
		if strings.HasSuffix(v, "v") && strings.ContainsAny(v, "+#") {
			s.report(lit.Pos(), "dump verb "+v)
		}
		if n < len(values) && carriesText(s.info.TypeOf(values[n])) && labelled.MatchString(before) {
			s.report(lit.Pos(), "label "+strings.TrimSpace(labelled.FindString(before)))
		}
		n++
	}
}

func (s *scanner) value(arg ast.Expr, t types.Type) {
	if t == nil {
		return
	}
	if d := dumpType(t); d != "" {
		s.report(arg.Pos(), "type "+d)
	}
	if !carriesText(t) {
		return
	}
	ast.Inspect(arg, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && sensitive.MatchString(id.Name) {
			s.report(id.Pos(), "name "+id.Name)
		}
		return true
	})
}
