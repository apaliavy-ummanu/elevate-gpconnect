package runtime

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/config"
	gpconnectHTTP "github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	//nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

func NewHTTPServer(config config.Config, server *gpconnectHTTP.Server) (*http.Server, error) {
	// --- load OpenAPI swagger for request validation ---
	//sw, _ := openapi.GetSwagger()

	// --- router ---
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// OpenAPI request validator (validates path/query/headers/body)
	//r.Use(nethttpmiddleware.OapiRequestValidator(sw)) // todo maybe custom implementation

	// API Key auth middleware (checks header: X-API-Key)
	//r.Use(apiKeyAuth(config.APIKey))

	// Mount all routes from the generated router
	openapi.HandlerFromMux(server, r)

	// --- http server + graceful shutdown ---
	srv := &http.Server{
		Addr:              ":" + config.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return srv, nil
}

// apiKeyAuth returns a middleware that validates X-API-Key if apiKey is non-empty.
// If API_KEY env var is unset, the middleware allows all requests (handy for local dev).
func apiKeyAuth(expected string) func(http.Handler) http.Handler {
	const hdr = "X-API-Key"
	return func(next http.Handler) http.Handler {
		if expected == "" {
			// no API key configured — skip enforcement
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get(hdr)
			if got == "" || got != expected {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`ApiKey header="%s"`, hdr))
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
