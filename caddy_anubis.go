package caddyanubis

import (
	"context"
	"net/http"

	"github.com/TecharoHQ/anubis"
	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(AnubisMiddleware{})
	httpcaddyfile.RegisterHandlerDirective("anubis", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("anubis", httpcaddyfile.After, "templates")
}

type AnubisMiddleware struct {
	Target       *string `json:"target,omitempty"`
	AnubisPolicy *policy.ParsedConfig
	AnubisServer *libanubis.Server

	logger *zap.Logger
}

type nextKey struct{}

// CaddyModule returns the Caddy module information.
func (AnubisMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.anubis",
		New: func() caddy.Module { return new(AnubisMiddleware) },
	}
}

// Provision implements caddy.Provisioner.
func (m *AnubisMiddleware) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger().Named("anubis")
	m.logger.Info("Anubis middleware provisioning")

	policy, err := libanubis.LoadPoliciesOrDefault("", anubis.DefaultDifficulty)
	if err != nil {
		return err
	}

	m.AnubisPolicy = policy

	m.AnubisServer, err = libanubis.New(libanubis.Options{
		Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.logger.Info("Anubis middleware calling next")

			if m.Target != nil {
				http.Redirect(w, r, *m.Target, http.StatusTemporaryRedirect)
			} else {
				if next, ok := r.Context().Value(nextKey{}).(caddyhttp.Handler); ok {
					next.ServeHTTP(w, r)
				}
			}
		}),
		Policy:         m.AnubisPolicy,
		ServeRobotsTXT: true,
	})
	if err != nil {
		return err
	}

	m.logger.Info("Anubis middleware provisioned")
	return nil
}

// Validate implements caddy.Validator.
func (m *AnubisMiddleware) Validate() error {
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *AnubisMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	m.logger.Info("Anubis middleware processing request")
	m.logger.Info("Anubis middleware sending request")

	// Inject the next handler into the context so the callback can retrieve it
	ctx := context.WithValue(r.Context(), nextKey{}, next)

	m.AnubisServer.ServeHTTP(w, r.WithContext(ctx))
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *AnubisMiddleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	// require an argument
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "target":
			if d.NextArg() {
				val := d.Val()
				m.Target = &val
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m AnubisMiddleware
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return &m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*AnubisMiddleware)(nil)
	_ caddy.Validator             = (*AnubisMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*AnubisMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*AnubisMiddleware)(nil)
)
