package http

import (
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/app/commands"
)

func mapUpdateRecordToPorts(in openapi.UpdateRecordRequest) commands.UpdateRecordCommand {
	return commands.UpdateRecordCommand{
		/*Patient: ports.PatientDTO{
			NHSNumber: deref(in.Patient.NhsNumber),
			// …
		},*/
		// …
	}
}
