package commands

import (
	"context"
)

type SendFHIRMessageCommand struct {
}

type SendFHIRMessageResult struct {
}

type SendFHIRMessageHandler interface {
	Handle(ctx context.Context, cmd SendFHIRMessageCommand) (message SendFHIRMessageResult, err error)
}

func NewSendFHIRMessageHandler() SendFHIRMessageHandler {
	return &sendFHIRMessageCmdHandler{}
}

type sendFHIRMessageCmdHandler struct {
}

func (h *sendFHIRMessageCmdHandler) Handle(ctx context.Context, cmd SendFHIRMessageCommand) (SendFHIRMessageResult, error) {
	return SendFHIRMessageResult{}, nil
}
