package rest

import (
	"compress/gzip"
	"net/http"

	"at.ourproject/energystore/internal/logging"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// uncompressed are the route templates whose answer is a file: xlsx is a zip archive already.
var uncompressed = map[string]bool{
	"/eeg/{ecid}/excel/report/download": true,
}

// HTTPHandler is the HTTP surface of router as served (server.go adds CORS around it): the access
// log inside, the compression outside. Inside, the gzip writer would start a 200 response while a
// panicking handler unwinds, and the access log could no longer answer 500.
func HTTPHandler(router *mux.Router) http.Handler {
	return compressJSON(router, logging.AccessLog(router))
}

// compressJSON gzips the answers of next for clients that accept it (gorilla's CompressHandler: gzip or
// deflate, whichever the client names first; no compression without the header or on an Upgrade
// request; Vary: Accept-Encoding), except for the file downloads in uncompressed, looked up on router.
// BestSpeed keeps the CPU cost per answer low.
func compressJSON(router *mux.Router, next http.Handler) http.Handler {
	compressed := handlers.CompressHandlerLevel(next, gzip.BestSpeed)
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
