package app

import (
	"context"

	commands2 "github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/commands"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/queries"
)

type CommandBus interface {
	UpdateRecord(ctx context.Context, cmd commands2.UpdateRecordCommand) (commands2.UpdateRecordResult, error)
	SendFHIRMessage(ctx context.Context, cmd commands2.SendFHIRMessageCommand) (commands2.SendFHIRMessageResult, error)
}

type QueryBus interface {
	GetMessageDetails(ctx context.Context, q queries.GetMessageDetailsQuery) (queries.GetMessageDetailsResult, error)
}

type commandBus struct {
	updateRecord    commands2.UpdateRecordHandler
	sendFHIRMessage commands2.SendFHIRMessageHandler
}

type queryBus struct {
	getMessageDetails queries.GetMessageDetailsQueryHandler
}

func NewCommandBus(
	update commands2.UpdateRecordHandler,
	send commands2.SendFHIRMessageHandler,
) CommandBus {
	return &commandBus{
		updateRecord:    update,
		sendFHIRMessage: send,
	}
}

func NewQueryBus(
	get queries.GetMessageDetailsQueryHandler,
) QueryBus {
	return &queryBus{
		getMessageDetails: get,
	}
}

func (b *commandBus) UpdateRecord(ctx context.Context, cmd commands2.UpdateRecordCommand) (commands2.UpdateRecordResult, error) {
	return b.updateRecord.Handle(ctx, cmd)
}

func (b *commandBus) SendFHIRMessage(ctx context.Context, cmd commands2.SendFHIRMessageCommand) (commands2.SendFHIRMessageResult, error) {
	return b.sendFHIRMessage.Handle(ctx, cmd)
}

func (b *queryBus) GetMessageDetails(ctx context.Context, q queries.GetMessageDetailsQuery) (queries.GetMessageDetailsResult, error) {
	return b.getMessageDetails.Handle(ctx, q)
}
