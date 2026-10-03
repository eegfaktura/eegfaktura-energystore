package tenantctx

import (
	"context"
	"testing"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
		arg  string
		ok   bool
	}{
		{"same tenant", WithCaller(context.Background(), "TE100001", false), "TE100001", true},
		{"case-insensitive", WithCaller(context.Background(), "te100001", false), "TE100001", true},
		{"foreign tenant", WithCaller(context.Background(), "TE100001", false), "TE100002", false},
		{"empty argument", WithCaller(context.Background(), "TE100001", false), "", false},
		{"empty header tenant", WithCaller(context.Background(), "", false), "", false},
		{"superuser any tenant", WithCaller(context.Background(), "TE100001", true), "TE100002", true},
		{"no caller in context", context.Background(), "TE100001", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Check(c.ctx, c.arg)
			if (err == nil) != c.ok {
				t.Fatalf("Check(%q) = %v, want ok=%v", c.arg, err, c.ok)
			}
		})
	}
}
