// Package apitest builds the HTTP surface of the service for tests (M2, reused by M6): the REST
// router plus /query (GraphQL) wired exactly as server.go does, tokens from the in-process fake
// Keycloak (fakeoidc, which must initialise before middleware — see its package comment) and
// loopback gRPC fakes for the mail server and the backend's master data.
package apitest

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"at.ourproject/energystore/graph"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"at.ourproject/energystore/middleware"
	protobuf "at.ourproject/energystore/protoc"
	"at.ourproject/energystore/rest"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// Router returns rest.NewRestServer() with /query behind GQLProtect, as server.go wires it
// (without the CORS wrapper).
func Router() *mux.Router {
	r := rest.NewRestServer()
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))
	r.Handle("/query", middleware.GQLProtect(srv))
	return r
}

// AppToken is a bearer token of the app realm with the given tenants and realm roles.
func AppToken(tenants []string, roles ...string) string {
	if roles == nil {
		roles = []string{}
	}
	return fakeoidc.Token(map[string]any{
		"tenant":       tenants,
		"realm_access": map[string]any{"roles": roles},
		"email":        "member@example.org",
	})
}

// Basic encodes user:password with standard base64, as HTTP clients send it (RFC 7617).
func Basic(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

// Request is one HTTP call against a handler.
type Request struct {
	Method, Path string
	Header       map[string]string
	Body         string
	ContentType  string
	RawBody      io.Reader
}

// StatusPanic is recorded when the handler panicked. net/http recovers a handler panic, logs it
// and drops the connection, so a client sees no status at all; the tests need a value.
const StatusPanic = 599

// Do runs req against h and returns the recorded response (StatusPanic if the handler panicked).
func Do(h http.Handler, req Request) (rec *httptest.ResponseRecorder) {
	var body io.Reader = bytes.NewBufferString(req.Body)
	if req.RawBody != nil {
		body = req.RawBody
	}
	r := httptest.NewRequest(req.Method, req.Path, body)
	ct := req.ContentType
	if ct == "" {
		ct = "application/json"
	}
	r.Header.Set("Content-Type", ct)
	for k, v := range req.Header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	defer func() {
		if p := recover(); p != nil {
			rec = httptest.NewRecorder()
			rec.Code = StatusPanic
			_, _ = fmt.Fprintf(rec.Body, "handler panicked: %v", p)
		}
	}()
	h.ServeHTTP(w, r)
	return w
}

// Fakes records the calls of the two gRPC services the HTTP layer reaches.
type Fakes struct {
	mu            sync.Mutex
	Mails         []*protobuf.SendExcelRequest
	MeterRequests []*protobuf.MeteringRequest
	Meters        []*protobuf.MeteringPoint

	protobuf.UnimplementedExcelAdminServiceServer
	protobuf.UnimplementedApiServiceServer
}

func (f *Fakes) SendExcel(_ context.Context, r *protobuf.SendExcelRequest) (*protobuf.SendExcelReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Mails = append(f.Mails, r)
	return &protobuf.SendExcelReply{}, nil
}

func (f *Fakes) MasterData_MeteringPoint(_ context.Context, r *protobuf.MeteringRequest) (*protobuf.MeteringPointReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.MeterRequests = append(f.MeterRequests, r)
	return &protobuf.MeteringPointReply{MeteringPoints: f.Meters}, nil
}

// MailCount is the number of SendExcel calls.
func (f *Fakes) MailCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Mails)
}

// StartFakes starts one loopback gRPC server with both services and points
// services.mail-server and services.master-server at it until the test ends.
func StartFakes(t testing.TB) *Fakes {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &Fakes{}
	s := grpc.NewServer()
	protobuf.RegisterExcelAdminServiceServer(s, f)
	protobuf.RegisterApiServiceServer(s, f)
	go func() { _ = s.Serve(lis) }()

	oldMail, oldMaster := viper.Get("services.mail-server"), viper.Get("services.master-server")
	viper.Set("services.mail-server", lis.Addr().String())
	viper.Set("services.master-server", lis.Addr().String())
	t.Cleanup(func() {
		s.Stop()
		viper.Set("services.mail-server", oldMail)
		viper.Set("services.master-server", oldMaster)
	})
	return f
}
