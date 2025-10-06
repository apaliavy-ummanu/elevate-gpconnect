deps:
	go mod download

OAPI_SPEC = api/http/openapi.yml
OAPI_PKG  = openapi
OAPI_OUT  = internal/adapters/http/openapi

oapi_codegen:
	# Models/types
	oapi-codegen -generate types \
		-o $(OAPI_OUT)/types.gen.go \
		-package $(OAPI_PKG) \
		$(OAPI_SPEC)

	# Chi server interfaces and handler wiring
	oapi-codegen -generate chi-server \
		-o $(OAPI_OUT)/chi_server.gen.go \
		-package $(OAPI_PKG) \
		$(OAPI_SPEC)

	# Typed HTTP client
	oapi-codegen -generate client \
		-o $(OAPI_OUT)/client.gen.go \
		-package $(OAPI_PKG) \
		$(OAPI_SPEC)

	# Embedded spec (produces GetSwagger())
	oapi-codegen -generate spec \
		-o $(OAPI_OUT)/spec.gen.go \
		-package $(OAPI_PKG) \
		$(OAPI_SPEC)

run:
	go run cmd/app/main.go