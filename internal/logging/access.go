package logging

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/gorilla/mux"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// The access line (logging concept point 6, logging-api-alt stage 2): one line per HTTP request,
// written after the response, with what the service's own token check decided. It names the
// community (tenant.id) and the Keycloak client (client.id), never a person: no user, e-mail,
// name, user agent, IP, query value or body (DSGVO, decided 2026-10-29).

type ctxKey struct{}

// requestInfo travels in the request context; the token check and the router fill it in.
type requestInfo struct {
	requestID, traceID, spanID string
	traceFlags                 string // of a valid incoming traceparent
	route, operation           string
	tenant, client             string
}

func idsFrom(ctx context.Context) *requestInfo {
	if ctx == nil {
		return nil
	}
	ri, _ := ctx.Value(ctxKey{}).(*requestInfo)
	return ri
}

// probes are not logged (noise).
var probes = map[string]bool{"/healthz": true, "/readyz": true}

var (
	requestIDPattern   = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	traceparentPattern = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-[0-9a-f]{2}$`)
)

// parseTraceparent accepts a W3C traceparent; version ff and all-zero ids are invalid.
func parseTraceparent(s string) (traceID, spanID string, ok bool) {
	m := traceparentPattern.FindStringSubmatch(s)
	if m == nil || m[1] == "ff" || strings.Trim(m[2], "0") == "" || strings.Trim(m[3], "0") == "" {
		return "", "", false
	}
	return m[2], m[3], true
}

// AccessLog wraps the whole HTTP surface. It takes X-Request-Id when it is a plain token (else
// generates one), reads trace.id/span.id from a W3C traceparent, answers with X-Request-Id and
// writes the access line after the handler returned.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if probes[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		ri := &requestInfo{requestID: r.Header.Get("X-Request-Id")}
		if !requestIDPattern.MatchString(ri.requestID) {
			ri.requestID = newRequestID()
		}
		if tp := r.Header.Get("traceparent"); tp != "" {
			if tid, sid, ok := parseTraceparent(tp); ok {
				ri.traceID, ri.spanID, ri.traceFlags = tid, sid, tp[len(tp)-2:]
			}
		}
		w.Header().Set("X-Request-Id", ri.requestID)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), ctxKey{}, ri)
		next.ServeHTTP(sw, r.WithContext(ctx))

		outcome, level := "success", slog.LevelInfo
		switch {
		case sw.status == http.StatusUnauthorized || sw.status == http.StatusForbidden:
			outcome, level = "refused", slog.LevelWarn
		case sw.status >= 400:
			outcome = "failure"
		}
		attrs := []slog.Attr{
			slog.String("event.action", "http.access"),
			slog.String("event.outcome", outcome),
			slog.String("http.request.method", r.Method),
			slog.Int("http.response.status_code", sw.status),
			slog.Int64("event.duration", time.Since(start).Nanoseconds()), // ECS: nanoseconds
		}
		route := "-" // unmatched: no http.route
		if ri.route != "" {
			route = ri.route
			attrs = append(attrs, slog.String("http.route", route))
		}
		if ri.operation != "" {
			attrs = append(attrs, slog.String("graphql.operation.name", ri.operation))
		}
		if ri.tenant != "" {
			attrs = append(attrs, slog.String("tenant.id", ri.tenant))
		}
		if ri.client != "" {
			attrs = append(attrs, slog.String("client.id", ri.client))
		}
		Log(ctx, 0, level, r.Method+" "+route+" "+strconv.Itoa(sw.status), attrs...)
	})
}

// MuxRoute is a gorilla/mux middleware that records the route template ("/eeg/v2/{ecid}/meta")
// for the access line, so no id of the path is logged.
func MuxRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ri := idsFrom(r.Context()); ri != nil {
			if rt := mux.CurrentRoute(r); rt != nil {
				ri.route, _ = rt.GetPathTemplate()
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SetCaller records what the token check decided: the tenant it checked and the Keycloak client
// (token azp, or the API client of the Basic-auth /query API). Never a user.
func SetCaller(ctx context.Context, tenant, client string) {
	if ri := idsFrom(ctx); ri != nil {
		ri.tenant, ri.client = tenant, client
	}
}

// GQLOperation is a gqlgen operation middleware (AroundOperations) that records the GraphQL
// operation name; without it every GraphQL call is just "/query". An anonymous operation leaves
// it unset (as in the backend).
func GQLOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	if ri := idsFrom(ctx); ri != nil && graphql.HasOperationContext(ctx) {
		oc := graphql.GetOperationContext(ctx)
		ri.operation = oc.OperationName
		if ri.operation == "" && oc.Operation != nil {
			ri.operation = oc.Operation.Name
		}
	}
	return next(ctx)
}

// GQLRecover is the gqlgen recover function (handler.Server.SetRecoverFunc): a panic in a
// resolver is one error line through this module instead of gqlgen's plain-text stack on stderr.
// The client gets the same generic error as before.
func GQLRecover(ctx context.Context, err any) error {
	Log(ctx, 0, slog.LevelError, "GraphQL resolver panicked",
		slog.String("event.action", "graphql.panic"),
		slog.String("error.type", fmt.Sprintf("%T", err)),
		slog.String("error.message", fmt.Sprint(err)),
		slog.String("error.stack_trace", string(debug.Stack())))
	return gqlerror.Errorf("internal system error")
}

// Propagate passes X-Request-Id and, when the request came with a traceparent, a child
// traceparent on to an outgoing request: the same trace id and flags, a new parent id. So the
// called service logs its own span.id, not the one energystore logs.
func Propagate(ctx context.Context, h http.Header) {
	ri := idsFrom(ctx)
	if ri == nil {
		return
	}
	h.Set("X-Request-Id", ri.requestID)
	if ri.traceID != "" {
		h.Set("traceparent", "00-"+ri.traceID+"-"+newSpanID()+"-"+ri.traceFlags)
	}
}

// newRequestID is a random UUID (version 4) as 32 hex characters.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return hex.EncodeToString(b[:])
}

// newSpanID is a random, non-zero 8-byte id as 16 hex characters.
func newSpanID() string {
	var b [8]byte
	for b == [8]byte{} {
		_, _ = rand.Read(b[:])
	}
	return hex.EncodeToString(b[:])
}

// statusWriter records the status code. It keeps Flush and Hijack (the GraphQL websocket transport
// needs Hijack); Unwrap lets http.ResponseController reach the original.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status, s.wroteHeader = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.wroteHeader = true
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := s.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
