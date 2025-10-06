package queries

import (
	"context"
)

type GetMessageDetailsQuery struct {
}

type GetMessageDetailsResult struct {
}

type GetMessageDetailsQueryHandler interface {
	Handle(ctx context.Context, query GetMessageDetailsQuery) (message GetMessageDetailsResult, err error)
}

func NewGetMessageDetailsQueryHandler() GetMessageDetailsQueryHandler {
	return &getMessageDetailsQueryHandler{}
}

type getMessageDetailsQueryHandler struct {
}

func (h *getMessageDetailsQueryHandler) Handle(ctx context.Context, query GetMessageDetailsQuery) (GetMessageDetailsResult, error) {
	return GetMessageDetailsResult{}, nil
}
