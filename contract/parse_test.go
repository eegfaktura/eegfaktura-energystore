package contract_test

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/stretchr/testify/require"
)

// TestMain: time.Local is already Europe/Vienna — fakeoidc (imported through apitest) sets it in its
// init() before its server goroutines start; writing it again here would race with them.
func TestMain(m *testing.M) {
	code := m.Run()
	fakeoidc.Close()
	os.Exit(code)
}

// call is one HTTP call of a neighbour, with its place in the copied source.
type call struct {
	caller, file string
	line         int
	method, path string // path template with {ecid}
}

func (c call) String() string {
	return c.caller + " " + c.file + ":" + strconv.Itoa(c.line) + " " + c.method + " " + c.path
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	require.NoError(t, sc.Err())
	return out
}

var (
	webFetch  = regexp.MustCompile("fetch\\(`\\$\\{ENERGY_API_SERVER\\}([^`]+)`")
	webMethod = regexp.MustCompile(`method:\s*'(\w+)'`)
	v3Call    = regexp.MustCompile(`\b(get|post)\("(/[^"]+)"`)
	v3Path    = regexp.MustCompile(`rest\.(get|post)\(\)`)
	v3UriPath = regexp.MustCompile(`\.path\("(/[^"]+)"\)`)
)

// webCalls reads every fetch of energy.service.ts that is not commented out.
func webCalls(t *testing.T) []call {
	src := lines(t, "testdata/web/energy.service.ts")
	var calls []call
	for i, l := range src {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		m := webFetch.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		method := "GET"
		for j := i + 1; j < len(src) && j < i+4; j++ {
			if mm := webMethod.FindStringSubmatch(src[j]); mm != nil {
				method = mm[1]
				break
			}
		}
		path := strings.ReplaceAll(m[1], "${tenant.ecId}", "{ecid}")
		calls = append(calls, call{caller: "eegfaktura-web", file: "src/service/energy.service.ts", line: i + 1, method: method, path: path})
	}
	return calls
}

// v3Calls reads every get/post of EnergyStoreClient.kt, including the raw call built with uri {}.
func v3Calls(t *testing.T) []call {
	src := lines(t, "testdata/v3/EnergyStoreClient.kt")
	var calls []call
	for i, l := range src {
		file := "backend/…/integration/energystore/EnergyStoreClient.kt"
		if m := v3Call.FindStringSubmatch(l); m != nil {
			calls = append(calls, call{caller: "eegfaktura-v3", file: file, line: i + 1, method: strings.ToUpper(m[1]), path: m[2]})
			continue
		}
		if m := v3Path.FindStringSubmatch(l); m != nil && i+1 < len(src) {
			if p := v3UriPath.FindStringSubmatch(src[i+1]); p != nil {
				calls = append(calls, call{caller: "eegfaktura-v3", file: file, line: i + 2, method: strings.ToUpper(m[1]), path: p[1]})
			}
		}
	}
	return calls
}

// kotlinField is one constructor parameter of a Kotlin data class and the JSON name it maps to.
type kotlinField struct{ Name, JSON, Type string }

// kotlinClasses parses the data classes of a Kotlin file: name → fields, and the line of each class.
func kotlinClasses(t *testing.T, path string) (map[string][]kotlinField, map[string]int) {
	src := strings.Join(lines(t, path), "\n")
	classes := map[string][]kotlinField{}
	at := map[string]int{}
	re := regexp.MustCompile(`data class (\w+)\(`)
	jsonName := regexp.MustCompile(`JsonProperty\("([^"]+)"\)`)
	param := regexp.MustCompile(`val (\w+):\s*([^=]+?)\s*(=.*)?$`)
	for _, m := range re.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		at[name] = strings.Count(src[:m[0]], "\n") + 1
		depth, start := 1, m[1]
		end := start
		for end < len(src) && depth > 0 {
			switch src[end] {
			case '(':
				depth++
			case ')':
				depth--
			}
			end++
		}
		body := src[start : end-1]
		var parts []string
		d, last := 0, 0
		for i, r := range body {
			switch r {
			case '(', '<':
				d++
			case ')', '>':
				d--
			case ',':
				if d == 0 {
					parts = append(parts, body[last:i])
					last = i + 1
				}
			}
		}
		parts = append(parts, body[last:])
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			pm := param.FindStringSubmatch(p[strings.LastIndex(p, "val "):])
			if pm == nil {
				continue
			}
			f := kotlinField{Name: pm[1], JSON: pm[1], Type: strings.TrimSpace(pm[2])}
			if jm := jsonName.FindStringSubmatch(p); jm != nil {
				f.JSON = jm[1]
			}
			classes[name] = append(classes[name], f)
		}
	}
	return classes, at
}
