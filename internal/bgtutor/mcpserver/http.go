package mcpserver

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPOptions configures the HTTP front end.
type HTTPOptions struct {
	// Token, when non-empty, is required on every /mcp request, either as
	// "Authorization: Bearer <token>" or as a "token" query parameter (some
	// connector UIs only accept a URL). Empty disables auth, which is only
	// sensible when listening on localhost.
	Token string
	// Logger receives request logs; nil discards them.
	Logger *slog.Logger
}

// Handler returns an http.Handler serving MCP at /mcp and a health check at
// /healthz. The MCP transport is Streamable HTTP in stateless mode with JSON
// responses, which suits a server that keeps no per-session state and sits
// behind a reverse proxy or tunnel.
//
// The SDK's DNS-rebinding protection rejects requests that reach a localhost
// listener with a non-localhost Host header. That is exactly what an HTTPS
// tunnel or local reverse proxy produces, so the protection is turned off when
// a token is configured: the token already stops a malicious web page from
// using the server, which is what the protection is for.
func Handler(server *mcp.Server, opts HTTPOptions) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless: true, JSONResponse: true, Logger: opts.Logger,
			DisableLocalhostProtection: opts.Token != "",
		},
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	var h http.Handler = mcpHandler
	if opts.Token != "" {
		h = requireToken(opts.Token, h)
	}
	mux.Handle("/mcp", h)
	return mux
}

// requireToken rejects requests that don't carry the expected token. The
// comparison is constant-time so the token can't be guessed by timing.
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("token")
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
			got = strings.TrimSpace(auth[7:])
		}
		if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="bgtutor"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
