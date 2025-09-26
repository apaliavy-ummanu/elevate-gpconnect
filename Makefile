deps:
	go mod download

oapi_codegen:
	mkdir -p client/http
	oapi-codegen -generate skip-prune,types -o client/http/types.gen.go -package http api/http/openapi.yml
	oapi-codegen -generate skip-prune,client -o client/http/client.gen.go -package http api/http/openapi.yml

run:
	go run cmd/app/main.go