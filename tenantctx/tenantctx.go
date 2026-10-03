// Package tenantctx traegt den von der Middleware geprueften Mandanten in den Request-Kontext.
//
// Die GraphQL-Resolver bekommen tenant als Argument vom Client. Gegen das Token geprueft ist aber nur
// der tenant-Header (GQLProtect). Ohne Abgleich koennte ein Nutzer mit Header A und Argument B die Daten
// einer fremden Gemeinschaft lesen und in ihren Speicher importieren. Das Paket haengt bewusst von
// nichts ab, damit graph es nutzen kann, ohne middleware (und dessen Keycloak-init) zu importieren.
package tenantctx

import (
	"context"
	"errors"
	"strings"
)

type callerKey struct{}

type caller struct {
	tenant    string
	superuser bool
}

// ErrTenantMismatch wird geliefert, wenn das tenant-Argument nicht zum geprueften Mandanten passt.
var ErrTenantMismatch = errors.New("tenant not permitted")

// WithCaller legt den geprueften Mandanten (aus dem Header) und die superuser-Rolle in den Kontext.
func WithCaller(ctx context.Context, tenant string, superuser bool) context.Context {
	return context.WithValue(ctx, callerKey{}, caller{tenant: tenant, superuser: superuser})
}

// Check prueft ein vom Client uebergebenes tenant-Argument gegen den geprueften Mandanten.
// Ohne Eintrag im Kontext wird abgelehnt (fail closed); superuser darf jeden Mandanten.
// Der Vergleich ist wie in der Middleware unabhaengig von Gross-/Kleinschreibung.
func Check(ctx context.Context, tenant string) error {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return ErrTenantMismatch
	}
	if c.superuser {
		return nil
	}
	if c.tenant == "" || !strings.EqualFold(c.tenant, tenant) {
		return ErrTenantMismatch
	}
	return nil
}
