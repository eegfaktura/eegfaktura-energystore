package rest

import (
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// uncompressed are the route templates whose answer is a file: xlsx is a zip archive already.
var uncompressed = map[string]bool{
	"/eeg/{ecid}/excel/report/download": true,
}

// CompressJSON gzips the answers of next for clients that send Accept-Encoding: gzip (gorilla's
// CompressHandler: Vary: Accept-Encoding always, no compression without the header or on an Upgrade
// request), except for the file downloads in uncompressed, looked up on router. It belongs outside
// logging.AccessLog: inside, the gzip writer would start a 200 response while a panicking handler
// unwinds, and the access log could no longer answer 500.
func CompressJSON(router *mux.Router, next http.Handler) http.Handler {
	compressed := handlers.CompressHandler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m mux.RouteMatch
		if router.Match(r, &m) && m.Route != nil {
			if tpl, _ := m.Route.GetPathTemplate(); uncompressed[tpl] {
				next.ServeHTTP(w, r)
				return
			}
		}
		compressed.ServeHTTP(w, r)
	})
}
