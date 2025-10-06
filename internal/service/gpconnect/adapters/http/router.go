package http

import (
	"context"
	"net/http"

	openapi2 "github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimw "github.com/oapi-codegen/nethttp-middleware"
)

func Router(srv *Server) http.Handler {
	swagger, _ := openapi2.GetSwagger()

	r := chi.NewRouter()

	// Add request validation middleware
	r.Use(oapimw.OapiRequestValidatorWithOptions(swagger, &oapimw.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(c context.Context, input *openapi3filter.AuthenticationInput) error {
				// API key auth here
				return nil
			},
		},
	}))

	openapi2.HandlerFromMux(srv, r) // generated chi-server binder

	return r
}
