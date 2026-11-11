package graph

import (
	"context"
	"errors"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport/logcapture"
	"at.ourproject/energystore/tenantctx"
	"github.com/99designs/gqlgen/graphql"
)

// Header TE100001 geprueft, Argument TE100002: beide Resolver lehnen ab, bevor sie einen Speicher oeffnen.
func TestResolversRejectForeignTenantArgument(t *testing.T) {
	r := &Resolver{}
	ctx := tenantctx.WithCaller(context.Background(), "TE100001", false)

	ok, err := r.Mutation().SingleUpload(ctx, "TE100002", "AT00999900000TC100200000000000002", "Sheet1", graphql.Upload{})
	if ok || !errors.Is(err, tenantctx.ErrTenantMismatch) {
		t.Fatalf("SingleUpload with a foreign tenant: ok=%v err=%v, want refused", ok, err)
	}

	date, err := r.Query().LastEnergyDate(ctx, "TE100002", "AT00999900000TC100200000000000002")
	if date != "" || !errors.Is(err, tenantctx.ErrTenantMismatch) {
		t.Fatalf("LastEnergyDate with a foreign tenant: date=%q err=%v, want refused", date, err)
	}
}

// Ohne geprueften Mandanten im Kontext (Resolver an GQLProtect vorbei verdrahtet) wird abgelehnt.
func TestResolversRejectWithoutCheckedTenant(t *testing.T) {
	r := &Resolver{}
	if _, err := r.Query().LastEnergyDate(context.Background(), "TE100001", "AT00999900000TC100200000000000002"); !errors.Is(err, tenantctx.ErrTenantMismatch) {
		t.Fatalf("LastEnergyDate without a checked tenant: err=%v, want refused", err)
	}
}

// The tenant argument is chosen by the client: a refusal says so, it does not repeat the value
// (dev#45 L2).
func TestRefusalLinesDoNotRepeatTheTenantArgument(t *testing.T) {
	out := logcapture.Capture(t)
	r := &Resolver{}
	ctx := tenantctx.WithCaller(context.Background(), "TE100001", false)
	const chosen = "CLIENT-CHOSEN-TENANT"

	_, _ = r.Mutation().SingleUpload(ctx, chosen, "AT00999900000TC100200000000000002", "Sheet1", graphql.Upload{})
	_, _ = r.Query().LastEnergyDate(ctx, chosen, "AT00999900000TC100200000000000002")

	if n := strings.Count(out.String(), "refused"); n != 2 {
		t.Fatalf("want both refusal lines, got %d: %s", n, out.String())
	}
	if strings.Contains(out.String(), chosen) {
		t.Fatalf("the tenant argument is in the log: %s", out.String())
	}
}
