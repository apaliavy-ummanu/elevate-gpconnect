package fhir

import (
	"strings"
	"time"

	fhirmodel "github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/fhir/model"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/gpconnect/adapters/http/openapi"
	"github.com/google/uuid"
)

const (
	xmlnsFHIR = "http://hl7.org/fhir"

	// Profiles
	profileITKMessageBundle       = "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-Message-Bundle-1"
	profileITKMessageHeader       = "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-MessageHeader-2"
	profileITKDocumentBundle      = "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-Document-Bundle-1"
	profileCareConnectComposition = "https://fhir.hl7.org.uk/STU3/StructureDefinition/CareConnect-Composition-1"
	profileGPCEncounter           = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Encounter-1"
	profileGPCOrganization        = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Organization-1"
	profileGPCPractitioner        = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Practitioner-1"
	profileGPCPractRole           = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-PractitionerRole-1"
	profileGPCPatient             = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Patient-1"
	profileGPCClinicalImpression  = "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-ClinicalImpression-1"

	// Code systems
	csITKMessageEvent = "https://fhir.nhs.uk/STU3/CodeSystem/ITK-MessageEvent-2" // ITK014M
	csRecipientType   = "https://fhir.nhs.uk/STU3/CodeSystem/ITK-RecipientType-1"
	csGPCParticipant  = "https://fhir.nhs.uk/STU3/CodeSystem/GPConnect-ParticipantType-1"

	// Local identifier systems
	sysBundleID      = "https://fhir.provider.example/identifier/bundle"
	sysCompositionID = "https://fhir.provider.example/identifier/composition"
	sysEncounterID   = "https://fhir.provider.example/identifier/encounter"
	sysClinImpID     = "https://fhir.provider.example/identifier/clinical-impression"
	sysStaffID       = "https://fhir.provider.example/identifier/staff"
	sysPatientLocal  = "https://fhir.provider.example/identifier/patient"

	// NHS systems
	sysODSOrg    = "https://fhir.nhs.uk/Id/ods-organization-code"
	sysSDSUserID = "https://fhir.nhs.uk/Id/sds-user-id"
	sysNHSNumber = "https://fhir.nhs.uk/Id/nhs-number"
)

type Options struct {
	SenderMeshMailbox string // MessageHeader.source.endpoint
	BusAckRequested   bool
	InfAckRequested   bool
	RecipientTypeCode string // e.g., "FI" For Information
}

type Composer struct {
}

func NewComposer() *Composer {
	return &Composer{}
}

func (c *Composer) BuildBundle(req openapi.UpdateRecordRequest, opt Options) (fhirmodel.Bundle, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Generate deterministic-ish UUIDs for the bundle content
	msgHeaderID := uuid.New()
	senderOrgID := uuid.New()
	docBundleID := uuid.New()
	compositionID := uuid.New()
	encounterID := uuid.New()
	practitionerID := uuid.New()
	practRoleID := uuid.New()
	patientID := uuid.New()
	clinImpID := uuid.New()

	// Helper URNs
	urnMsgHeader := urn(msgHeaderID)
	urnSenderOrg := urn(senderOrgID)
	urnDocBundle := urn(docBundleID)
	urnComposition := urn(compositionID)
	urnEncounter := urn(encounterID)
	urnPractitioner := urn(practitionerID)
	urnPractRole := urn(practRoleID)
	urnPatient := urn(patientID)
	urnClinImp := urn(clinImpID)

	// ------------------------------
	// Outer ITK Message Bundle
	// ------------------------------
	outer := fhirmodel.Bundle{
		Xmlns: xmlnsFHIR,
		ID:    attr(msgHeaderID.String()),
		Meta: &fhirmodel.Meta{
			LastUpdated: attr(now),
			Profile:     attr(profileITKMessageBundle),
		},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysBundleID),
			Value:  attr(msgHeaderID.String()),
		},
		Type: attr("message"),
	}

	// MessageHeader with ITK handling extension
	recipientCode := defaultIfEmpty(opt.RecipientTypeCode, "FI")

	mhChildren := []fhirmodel.MHSubExtension{
		{
			URL:          "BusAckRequested",
			ValueBoolean: pbool(opt.BusAckRequested),
		},
		{
			URL:          "InfAckRequested",
			ValueBoolean: pbool(opt.InfAckRequested),
		},
		{
			URL: "RecipientType",
			ValueCoding: &fhirmodel.ValueCoding{
				System:  attr(csRecipientType),
				Code:    attr(recipientCode),
				Display: optDisplay(recipientCode), // pointer is fine to be nil
			},
		},
		{
			URL: "MessageDefinition",
			ValueRef: &fhirmodel.ValueReference{
				Reference: fhirmodel.Reference{Reference: attr("https://fhir.nhs.uk/MessageDefinition")},
			},
		},
		{
			URL:         "SenderReference",
			ValueString: &fhirmodel.Text{Value: urnEncounter}, // or a specific sender URN if you prefer
		},
		{
			URL:         "LocalExtension",
			ValueString: &fhirmodel.Text{Value: "None"},
		},
	}

	msgHeader := &fhirmodel.MessageHeader{
		ID:   attr(msgHeaderID.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileITKMessageHeader)},
		Extension: &fhirmodel.MessageHandlingExtension{
			Extensions: mhChildren,
		},
		Event: &fhirmodel.CodingEvent{
			System:  attr(csITKMessageEvent),
			Code:    attr("ITK014M"),
			Display: attr("ITK Update Record"),
		},
		Sender:    &fhirmodel.Reference{Reference: attr(urnSenderOrg)},
		Timestamp: attr(now),
		Source:    &fhirmodel.MessageSource{Endpoint: attr(opt.SenderMeshMailbox)},
		Focus:     &fhirmodel.Reference{Reference: attr(urnDocBundle)},
	}

	outer.Entry = append(outer.Entry,
		entry(urnMsgHeader, fhirmodel.EntryResource{MessageHeader: msgHeader}),
		entry(urnSenderOrg, fhirmodel.EntryResource{Organization: orgFromSender(req.Sender, senderOrgID, now)}),
	)

	// ------------------------------
	// Inner ITK Document Bundle
	// ------------------------------
	inner := &fhirmodel.Bundle{
		ID:   attr(docBundleID.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileITKDocumentBundle)},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysBundleID),
			Value:  attr(docBundleID.String()),
		},
		Type: attr("document"),
	}

	// Practitioner (author/recorder)
	prac := practitionerFrom(req.Practitioner, practitionerID, now)

	// Patient
	pat := patientFrom(req.Patient, patientID, now)

	// Encounter
	pract := "urn:uuid:" + practitionerID.String()

	enc := encounterFrom(req.Encounter, encounterID, urnPatient, req, now, pract, urnSenderOrg)

	// Composition (links to patient/encounter/practitioner)
	comp := compositionFrom(&req.Composition, compositionID, urnPatient, urnEncounter, urnPractitioner, now)

	// Clinical Impression (optional)
	var clinImp *fhirmodel.ClinicalImpression
	if req.ClinicalImpression.Summary != nil {
		clinImp = clinicalImpressionFrom(req.ClinicalImpression, clinImpID, urnPatient, urnEncounter, urnPractitioner, now)
	}

	// Wire entries into inner bundle (and composition.section)
	inner.Entry = append(inner.Entry,
		entry(urnComposition, fhirmodel.EntryResource{Composition: comp}),
		entry(urnEncounter, fhirmodel.EntryResource{Encounter: enc}),
		entry(urnSenderOrg, fhirmodel.EntryResource{Organization: orgFromSender(req.Sender, uuid.Nil, now)}), // serviceProvider org
		entry(urnPractitioner, fhirmodel.EntryResource{Practitioner: prac}),
	)

	// PractitionerRole if you populate it (optional; minimal stub here)
	prRole := practitionerRoleFrom(practitionerID, req.Organisation, practRoleID, now, urnPractitioner, urnSenderOrg)
	if prRole != nil {
		inner.Entry = append(inner.Entry, entry(urnPractRole, fhirmodel.EntryResource{PractitionerRole: prRole}))
	}

	inner.Entry = append(inner.Entry,
		entry(urnPatient, fhirmodel.EntryResource{Patient: pat}),
	)

	// ClinicalImpression (optional)
	if clinImp != nil {
		inner.Entry = append(inner.Entry, entry(urnClinImp, fhirmodel.EntryResource{ClinicalImpression: clinImp}))
		// Ensure Composition.section points to resources of interest
		comp.Section.Entry = append(comp.Section.Entry,
			&fhirmodel.Reference{Reference: attr(urnEncounter)},
			&fhirmodel.Reference{Reference: attr(urnSenderOrg)},
			&fhirmodel.Reference{Reference: attr(urnPractitioner)},
			&fhirmodel.Reference{Reference: attr(urnPatient)},
			&fhirmodel.Reference{Reference: attr(urnClinImp)},
		)
	}

	// Add inner bundle as an entry in the outer bundle focus chain
	outer.Entry = append(outer.Entry, entry(urnDocBundle, fhirmodel.EntryResource{Bundle: inner}))

	return outer, nil
}

// ==============================
// Mapping helpers
// ==============================

// entry helper (FIX for unresolved reference)
func entry(fullURL string, res fhirmodel.EntryResource) fhirmodel.BundleEntry {
	return fhirmodel.BundleEntry{
		FullURL:  attr(fullURL),
		Resource: res,
	}
}

func orgFromSender(s openapi.Sender, forcedID uuid.UUID, now string) *fhirmodel.Organization {
	id := forcedID
	if id == uuid.Nil {
		id = uuid.New()
	}
	return &fhirmodel.Organization{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCOrganization)},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysODSOrg),
			Value:  attr(s.OdsCode),
		},
	}
}

func orgFromOrganisation(o openapi.Organisation, forcedID uuid.UUID, now string) *fhirmodel.Organization {
	id := forcedID
	if id == uuid.Nil {
		id = uuid.New()
	}
	org := &fhirmodel.Organization{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCOrganization)},
	}
	if o.Identifier.Value != "" {
		org.Identifier = &fhirmodel.Identifier{
			System: attr(o.Identifier.System),
			Value:  attr(o.Identifier.Value),
		}
	}
	if o.Name != "" {
		org.Name = attr(o.Name)
	}
	return org
}

// --- replace practitionerFrom:
func practitionerFrom(p openapi.Practitioner, id uuid.UUID, now string) *fhirmodel.Practitioner {
	out := &fhirmodel.Practitioner{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCPractitioner)},
	}

	// Identifier is a CONCRETE struct in your OpenAPI model
	if p.Identifier.System != "" || p.Identifier.Value != "" {
		out.Identifier = &fhirmodel.Identifier{
			System: attr(p.Identifier.System),
			Value:  attr(p.Identifier.Value),
		}
	} else {
		out.Identifier = &fhirmodel.Identifier{
			System: attr(sysSDSUserID),
			Value:  attr(id.String()),
		}
	}

	// Name is also a CONCRETE struct
	if (openapi.Name{}) != p.Name {
		out.Name = &fhirmodel.HumanName{
			Use:    strAttr(p.Name.Use),
			Family: strAttr(p.Name.Family),
			Given:  strAttr(p.Name.Given),
			Prefix: strAttr(p.Name.Prefix),
		}
	}
	out.Gender = strAttr(p.Gender)
	return out
}

func patientFrom(p openapi.Patient, id uuid.UUID, now string) *fhirmodel.Patient {
	out := &fhirmodel.Patient{
		ID:         attr(id.String()),
		Meta:       &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCPatient)},
		Identifier: []*fhirmodel.Identifier{},
	}
	// NHS number
	if p.NhsNumber != "" {
		out.Identifier = append(out.Identifier, &fhirmodel.Identifier{
			System: attr(sysNHSNumber),
			Value:  attr(p.NhsNumber),
		})
	}
	// Local identifier
	out.Identifier = append(out.Identifier, &fhirmodel.Identifier{
		System: attr(sysPatientLocal),
		Value:  attr(id.String()),
	})

	// Name is concrete struct
	if (openapi.Name{}) != p.Name {
		out.Name = &fhirmodel.HumanName{
			Use:    strAttr(p.Name.Use),
			Family: strAttr(p.Name.Family),
			Given:  strAttr(p.Name.Given),
			Prefix: strAttr(p.Name.Prefix),
		}
	}
	if p.Gender != nil {
		out.Gender = strAttr(p.Gender)
	}
	if !p.DateOfBirth.IsZero() {
		out.BirthDate = attr(p.DateOfBirth.Time.Format("2006-01-02"))
	}
	return out
}

// --- replace encounterFrom signature & body:
func encounterFrom(
	e openapi.Encounter,
	id uuid.UUID,
	urnPatient string,
	req openapi.UpdateRecordRequest,
	now string,
	urnPractitioner string,
	urnServiceProvider string, // NEW: sender org URN
) *fhirmodel.Encounter {
	out := &fhirmodel.Encounter{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCEncounter)},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysEncounterID),
			Value:  attr(urn(id)),
		},
		Status: attr(defaultIfEmpty(string(e.Status), "finished")),
		Type: &fhirmodel.CodeableConcept{
			Coding: &fhirmodel.Coding{
				System:  attr(e.EncounterType.System),
				Code:    attr(e.EncounterType.Code),
				Display: strAttr(e.EncounterType.Display),
			},
			Text: strAttr(e.EncounterType.Display),
		},
		Subject:     &fhirmodel.Reference{Reference: attr(urnPatient)},
		Participant: []*fhirmodel.EncounterParticipant{},
	}

	// Period (*time.Time -> RFC3339)
	if e.Period != nil {
		pp := &fhirmodel.Period{}
		if e.Period.Start != nil {
			pp.Start = attr(e.Period.Start.UTC().Format(time.RFC3339))
		}
		if e.Period.End != nil {
			pp.End = attr(e.Period.End.UTC().Format(time.RFC3339))
		}
		out.Period = pp
	}

	// Participants → point at our known Practitioner for now
	for _, p := range e.Participants {
		code := strings.ToUpper(string(p.Role))
		if code == "" {
			code = "PART"
		}
		ep := &fhirmodel.EncounterParticipant{
			Type: &fhirmodel.CodeableConcept{
				Coding: &fhirmodel.Coding{
					System:  attr(csGPCParticipant),
					Code:    attr(code),
					Display: attr(participantDisplay(code)),
				},
				Text: attr(strings.ToLower(participantDisplay(code))),
			},
			Individual: &fhirmodel.Reference{Reference: attr(urnPractitioner)},
		}
		if p.Period != nil {
			ep.Period = &fhirmodel.Period{}
			if p.Period.Start != nil {
				ep.Period.Start = attr(p.Period.Start.UTC().Format(time.RFC3339))
			}
			if p.Period.End != nil {
				ep.Period.End = attr(p.Period.End.UTC().Format(time.RFC3339))
			}
		}
		out.Participant = append(out.Participant, ep)
	}

	// Reason from composition type (handle concrete struct, not pointer)
	if req.Composition.Type.System != "" {
		out.Reason = &fhirmodel.CodeableConcept{
			Coding: &fhirmodel.Coding{
				System:  attr(req.Composition.Type.System),
				Code:    attr(req.Composition.Type.Code),
				Display: strAttr(req.Composition.Type.Display),
			},
			Text: strAttr(req.Composition.Type.Display),
		}
	}

	// ServiceProvider → explicit sender org URN
	out.ServiceProvider = &fhirmodel.Reference{Reference: attr(urnServiceProvider)}
	return out
}

// --- replace compositionFrom (Type is CONCRETE):
func compositionFrom(c *openapi.CompositionDetails, id uuid.UUID, urnPatient, urnEncounter, urnPract string, now string) *fhirmodel.Composition {
	var codingSystem, codingCode, codingDisplay, typeText, title string
	if c != nil {
		title = resolveStrPtr(c.Title)
		codingSystem = c.Type.System
		codingCode = c.Type.Code
		codingDisplay = resolveStrPtr(c.Type.Display)
		typeText = resolveStrPtr(c.Type.Text)
	}
	return &fhirmodel.Composition{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileCareConnectComposition)},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysCompositionID),
			Value:  attr(id.String()),
		},
		Status: attr("final"),
		Type: &fhirmodel.CodeableConcept{
			Coding: &fhirmodel.Coding{
				System:  attr(codingSystem),
				Code:    attr(codingCode),
				Display: attr(codingDisplay),
			},
			Text: attr(typeText),
		},
		Subject:   &fhirmodel.Reference{Reference: attr(urnPatient)},
		Encounter: &fhirmodel.Reference{Reference: attr(urnEncounter)},
		Date:      attr(time.Now().Format("2006-01-02")),
		Author:    []*fhirmodel.Reference{{Reference: attr(urnPract)}},
		Title:     attr(title),
		Section:   &fhirmodel.CompositionSection{Entry: []*fhirmodel.Reference{}},
	}
}

func clinicalImpressionFrom(
	ci openapi.ClinicalImpression,
	id uuid.UUID,
	urnPatient, urnEncounter, urnAssessor string,
	now string,
) *fhirmodel.ClinicalImpression {
	out := &fhirmodel.ClinicalImpression{
		ID:   attr(id.String()),
		Meta: &fhirmodel.Meta{Profile: attr(profileGPCClinicalImpression)},
		Identifier: &fhirmodel.Identifier{
			System: attr(sysClinImpID),
			Value:  attr(id.String()),
		},
		Subject:     &fhirmodel.Reference{Reference: attr(urnPatient)},
		Context:     &fhirmodel.Reference{Reference: attr(urnEncounter)},
		Date:        attr(time.Now().Format("2006-01-02")),
		Assessor:    &fhirmodel.Reference{Reference: attr(urnAssessor)},
		Summary:     strAttr(ci.Summary),
		Description: strAttr(ci.Description),
		Problem:     []*fhirmodel.CodeableConcept{},
		Finding:     []*fhirmodel.ClinicalImpressionFinding{},
	}

	if ci.Status != nil {
		out.Status = attr(defaultIfEmpty(string(*ci.Status), "completed"))
	}

	// ---- Problems ----
	// Supports both []openapi.CodeableConcept and *[], depending on generator.
	if ci.Problem != nil {
		// pointer to slice
		for _, pr := range *ci.Problem {
			out.Problem = append(out.Problem, &fhirmodel.CodeableConcept{
				Coding: &fhirmodel.Coding{
					System:  attr(pr.System),
					Code:    attr(pr.Code),
					Display: strAttr(pr.Display),
				},
				Text: strAttr(pr.Display),
			})
		}
	} else if ps, ok := any(ci.Problem).([]openapi.CodeableConcept); ok {
		// non-pointer slice (some generators)
		for _, pr := range ps {
			out.Problem = append(out.Problem, &fhirmodel.CodeableConcept{
				Coding: &fhirmodel.Coding{
					System:  attr(pr.System),
					Code:    attr(pr.Code),
					Display: strAttr(pr.Display),
				},
				Text: strAttr(pr.Display),
			})
		}
	}

	// ---- Findings ----
	if ci.Finding != nil {
		for _, f := range *ci.Finding { // NOTE: dereference pointer to slice
			out.Finding = append(out.Finding, &fhirmodel.ClinicalImpressionFinding{
				Item: &fhirmodel.CodeableConcept{
					Coding: &fhirmodel.Coding{
						System:  attr(f.Item.System),
						Code:    attr(f.Item.Code),
						Display: strAttr(f.Item.Display),
					},
					Text: strAttr(f.Item.Display),
				},
				Basis: strAttr(f.Basis),
			})
		}
	} else if fs, ok := any(ci.Finding).([]openapi.ClinicalImpressionFinding); ok {
		// fallback if generator made it a plain slice
		for _, f := range fs {
			out.Finding = append(out.Finding, &fhirmodel.ClinicalImpressionFinding{
				Item: &fhirmodel.CodeableConcept{
					Coding: &fhirmodel.Coding{
						System:  attr(f.Item.System),
						Code:    attr(f.Item.Code),
						Display: strAttr(f.Item.Display),
					},
					Text: strAttr(f.Item.Display),
				},
				Basis: strAttr(f.Basis),
			})
		}
	}

	return out
}

func practitionerRoleFrom(practID uuid.UUID, org openapi.Organisation, prRoleID uuid.UUID, now, urnPract, urnOrg string) *fhirmodel.PractitionerRole {
	return &fhirmodel.PractitionerRole{
		ID:           attr(prRoleID.String()),
		Meta:         &fhirmodel.Meta{LastUpdated: attr(now), Profile: attr(profileGPCPractRole)},
		Practitioner: &fhirmodel.Reference{Reference: attr(urnPract)},
		Organization: &fhirmodel.Reference{Reference: attr(urnOrg)},
	}
}

// ==============================
// XML helpers
// ==============================

func urn(id uuid.UUID) string { return "urn:uuid:" + id.String() }
func urnStr(id string) string { return "urn:uuid:" + id }

func attr(v string) *fhirmodel.Attr {
	if v == "" {
		return nil
	}
	return &fhirmodel.Attr{Value: v}
}
func strAttr(v *string) *fhirmodel.Attr {
	if v == nil {
		return nil
	}
	return &fhirmodel.Attr{Value: *v}
}
func boolAttr(b bool) *fhirmodel.Attr {
	if b {
		return &fhirmodel.Attr{Value: "true"}
	}
	return &fhirmodel.Attr{Value: "false"}
}
func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
func participantDisplay(code string) string {
	switch strings.ToUpper(code) {
	case "REC":
		return "recorder"
	case "PPRF":
		return "primary performer"
	default:
		return "participant"
	}
}
func displayForRecipient(code string) string {
	switch strings.ToUpper(code) {
	case "FI":
		return "For Information"
	case "FA":
		return "For Action"
	default:
		return "For Information"
	}
}
func resolveStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pbool(b bool) *bool { return &b }

func optDisplay(code string) *fhirmodel.Attr {
	// map "FI" → "For Information" etc. Adjust as needed.
	switch code {
	case "FI":
		v := fhirmodel.Attr{Value: "For Information"}
		return &v
	default:
		return nil
	}
}
