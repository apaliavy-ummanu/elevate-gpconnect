package http

import (
	"encoding/json"
	"net/http"

	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/commands"
)

type Server struct {
	cmdBus   app.CommandBus
	queryBus app.QueryBus
}

func NewServer(cmdBus app.CommandBus, queryBus app.QueryBus) *Server {
	return &Server{
		cmdBus:   cmdBus,
		queryBus: queryBus,
	}
}

func (s *Server) SendUpdateRecordMessage(w http.ResponseWriter, r *http.Request, params openapi.SendUpdateRecordMessageParams) {
	var in openapi.UpdateRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	cmd := commands.UpdateRecordCommand{
		Request: in,
	}

	result, err := s.cmdBus.UpdateRecord(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	_ = json.NewEncoder(w).Encode(result.Message)
}

func (s *Server) SendFHIRMessage(w http.ResponseWriter, r *http.Request, params openapi.SendFHIRMessageParams) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) GetMessageById(w http.ResponseWriter, r *http.Request, messageId string) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	//TODO implement me
	panic("implement me")
}
