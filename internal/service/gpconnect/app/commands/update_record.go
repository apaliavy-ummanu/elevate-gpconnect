package commands

import (
	"context"
	"encoding/xml"

	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/fhir"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/fhir/model"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
)

// todo: we shouldn't use http models here, this is just for poc
type UpdateRecordCommand struct {
	Request openapi.UpdateRecordRequest
}

type UpdateRecordResult struct {
	Message string // todo this is a temporary solution, just to render xml
}

type UpdateRecordHandler interface {
	Handle(ctx context.Context, cmd UpdateRecordCommand) (result UpdateRecordResult, err error)
}

func NewUpdateRecordHandler(fhirComposer *fhir.Composer) UpdateRecordHandler {
	return &updateRecordCmdHandler{
		fhirComposer: fhirComposer,
	}
}

type updateRecordCmdHandler struct {
	fhirComposer *fhir.Composer
}

func (h *updateRecordCmdHandler) Handle(ctx context.Context, cmd UpdateRecordCommand) (UpdateRecordResult, error) {
	// 1) compose in-memory FHIR models_deprecated
	bundle, err := h.fhirComposer.BuildBundle(cmd.Request, fhir.Options{})
	if err != nil {
		return UpdateRecordResult{}, err
	}

	// 2 - make a separate marshaller service??
	withNS := namespaced(bundle)
	message, err := xml.MarshalIndent(withNS, "", "  ")
	if err != nil {
		return UpdateRecordResult{}, err
	}

	// 3 - send message to MESH

	return UpdateRecordResult{
		Message: string(message),
	}, nil
}

// todo move code below to other place
type namespacedBundle struct {
	XMLName xml.Name `xml:"Bundle"`
	XMLNS   string   `xml:"xmlns,attr"`
	model.Bundle
}

const fhirNS = "http://hl7.org/fhir"

func namespaced(b model.Bundle) namespacedBundle {
	return namespacedBundle{XMLNS: fhirNS, Bundle: b}
}
