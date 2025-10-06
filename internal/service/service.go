package service

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/config"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/fhir"
	gpconnectHTTP "github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/commands"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/queries"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/runtime"
)

type Service struct {
	httpServer *http.Server
}

func NewGPConnectService() (*Service, error) {
	appConfig, err := config.NewConfigFromEnv()
	if err != nil {
		return nil, err
	}

	// todo logger

	// init composer
	composer := fhir.NewComposer()
	
	// init commands
	updateHandler := commands.NewUpdateRecordHandler(composer)
	sendFHIRHandler := commands.NewSendFHIRMessageHandler()
	cmdBus := app.NewCommandBus(updateHandler, sendFHIRHandler)

	// init queries
	getMessageDetailsHandler := queries.NewGetMessageDetailsQueryHandler()
	queryBus := app.NewQueryBus(getMessageDetailsHandler)

	// init http handler
	gpconnectHTTPServer := gpconnectHTTP.NewServer(cmdBus, queryBus)

	httpServer, err := runtime.NewHTTPServer(appConfig, gpconnectHTTPServer)
	if err != nil {
		return nil, err
	}

	return &Service{
		httpServer: httpServer,
	}, nil
}

// todo pass ctx properly
func (s *Service) Start(ctx context.Context) error {
	go func() {
		//log.Printf("Listening on %s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// wait for SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Println("Shutting down...")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(timeoutCtx); err != nil {
		return err
	}

	log.Println("Server stopped.")

	return nil
}
