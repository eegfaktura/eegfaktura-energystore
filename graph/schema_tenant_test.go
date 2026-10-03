package graph

import (
	"context"
	"errors"
	"testing"

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
