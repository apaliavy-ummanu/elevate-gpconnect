package main

import (
	"context"

	"github.com/Cleo-Systems/elevate-gpconnect/internal/service"
)

func main() {
	ctx := context.Background()

	svc, err := service.NewGPConnectService()
	if err != nil {
		panic(err)
	}

	err = svc.Start(ctx)
	if err != nil {
		panic(err)
	}
}
