package common

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cleo-Systems/elevate-gpconnect/client/http"
	"github.com/google/uuid"
)

/* ------------ Public API ------------- */

type Config struct {
	SenderMeshMailbox                 string
	DefaultSenderODS                  string
	DefaultBusinessAckRequested       bool
	DefaultInfrastructureAckRequested bool
	DefaultRecipientType              string // e.g. "FI"
}

func BuildUpdateRecordFHIRXML(req http.UpdateRecordRequest, cfg Config) ([]byte, error) {
	if err := validateMinimal(req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	lastUpdated := now.Format(time.RFC3339Nano)

	// IDs (urn:uuid)
	msgHeaderID := newURN()
	headerOrgID := newURN()
	docBundleID := newURN()
	compID := newURN()
	patientID := newURN()
	practID := newURN()
	practRoleID := newURN()
	encPrimaryID := newURN()

	// sender ODS
	senderODS := cfg.DefaultSenderODS
	if req.Encounter != nil && req.Encounter.PerformerODS != nil {
		senderODS = *req.Encounter.PerformerODS
	}
	if req.Encounters != nil {
		for _, e := range *req.Encounters {
			if e.Role != nil && *e.Role == "primary" && e.PerformerODS != nil {
				senderODS = *e.PerformerODS
				break
			}
		}
	}

	// Build document bundle entries (order: Composition first)
	var docEntries []Entry

	// Patient
	patient := makePatient(patientID, req.Patient, lastUpdated)
	docEntries = append(docEntries, Entry{FullURL: patientID, Resource: EntryResource{Patient: &patient}})

	// Org (service provider)
	orgDocID := newURN()
	orgDoc := makeOrganization(orgDocID, "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Organization-1", senderODS, lastUpdated)
	docEntries = append(docEntries, Entry{FullURL: orgDocID, Resource: EntryResource{Organization: &orgDoc}})

	// Practitioner
	// todo
	pr := makePractitioner(practID, req.Provenance.Author, lastUpdated)
	docEntries = append(docEntries, Entry{FullURL: practID, Resource: EntryResource{Practitioner: &pr}})

	// PractitionerRole (optional when role provided)
	if req.Provenance.Author.Role != nil && req.Provenance.Author.Role.System != "" && req.Provenance.Author.Role.Code != "" {
		prRole := makePractitionerRole(practRoleID, practID, orgDocID, *req.Provenance.Author.Role, lastUpdated)
		docEntries = append(docEntries, Entry{FullURL: practRoleID, Resource: EntryResource{PractitionerRole: &prRole}})
	}

	// Encounters (choose primary)
	primary, related := resolveEncounters(req)
	encPrimary := makeEncounter(encPrimaryID, primary, patientID, practID, orgDocID, lastUpdated)
	docEntries = append(docEntries, Entry{FullURL: encPrimaryID, Resource: EntryResource{Encounter: &encPrimary}})
	for range related {
		relID := newURN()
		e := makeEncounter(relID, primary, patientID, practID, orgDocID, lastUpdated) // clone shape; adjust if you carry distinct data
		docEntries = append(docEntries, Entry{FullURL: relID, Resource: EntryResource{Encounter: &e}})
	}

	// Observations
	if req.Observations != nil {
		for _, ob := range *req.Observations {
			oid := newURN()
			obs := makeObservation(oid, ob, patientID, encPrimaryID, practID, lastUpdated)
			docEntries = append(docEntries, Entry{FullURL: oid, Resource: EntryResource{Observation: &obs}})
		}
	}

	// Narrative sections -> ClinicalImpression
	if req.NarrativeSections != nil {
		for _, nb := range *req.NarrativeSections {
			cid := newURN()
			ci := makeClinicalImpression(cid, nb, patientID, encPrimaryID, practID, lastUpdated)
			docEntries = append(docEntries, Entry{FullURL: cid, Resource: EntryResource{ClinicalImpression: &ci}})
		}
	}

	// Attachments -> DocumentReference
	if req.Attachments != nil {
		/*for _, att := range *req.Attachments {
			if _, err := base64.StdEncoding.DecodeString(att.Base64); err != nil {
				return nil, fmt.Errorf("attachment %q is not valid base64: %w", att.Title, err)
			}
			drID := newURN()
			dr := makeDocumentReference(drID, att, patientID, lastUpdated)
			//docEntries = append(docEntries, Entry{FullURL: drID, Resource: EntryResource{DocumentBundle: &dr}})
		}*/ // todo
	}

	// NEW: medications
	if req.ClinicalSummary.MedicationsSupplied != nil && len(*req.ClinicalSummary.MedicationsSupplied) > 0 {
		for _, ms := range *req.ClinicalSummary.MedicationsSupplied {
			mdEntry, _ := makeMedicationDispense(ms, patientID, encPrimaryID, practID, lastUpdated)
			docEntries = append(docEntries, mdEntry)
		}
	}

	// Composition (first entry in document bundle)
	comp := makeComposition(compID, req, patientID, encPrimaryID, practID, lastUpdated)
	docEntries = append([]Entry{{FullURL: compID, Resource: EntryResource{Composition: &comp}}}, docEntries...)

	// Inner document Bundle
	docBundle := Bundle{
		XMLName: xml.Name{Local: "Bundle"},
		ID:      Attr{Value: trimURN(docBundleID)},
		Meta:    Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-Document-Bundle-1"}},
		Identifier: Identifier{
			System: Attr{Value: "https://fhir.provider.example/identifier/bundle"},
			Value:  Attr{Value: trimURN(docBundleID)},
		},
		Type:  Text{Value: "document"},
		Entry: docEntries,
	}

	// Header Organization for MessageHeader.sender
	headerOrg := makeOrganization(headerOrgID, "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-ITK-Header-Organization-1", senderODS, lastUpdated)

	// MessageHeader
	msgHeader := makeMessageHeader(msgHeaderID, docBundleID, headerOrgID, cfg, req.MessageHeaderOptions, lastUpdated)

	// Outer message Bundle
	msgBundle := Bundle{
		XMLName: xml.Name{Local: "Bundle"},
		ID:      Attr{Value: "gpconnect-update-record"},
		Meta:    Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-Message-Bundle-1"}},
		Identifier: Identifier{
			System: Attr{Value: "https://fhir.provider.example/identifier/bundle"},
			Value:  Attr{Value: "gpconnect-update-record"},
		},
		Type: Text{Value: "message"},
		Entry: []Entry{
			{FullURL: msgHeaderID, Resource: EntryResource{MessageHeader: &msgHeader}},
			{FullURL: headerOrgID, Resource: EntryResource{Organization: &headerOrg}},
			{FullURL: docBundleID, Resource: EntryResource{DocumentBundle: &docBundle}},
		},
	}

	// Marshal with the FHIR namespace
	withNS := namespaced(msgBundle)
	return xml.MarshalIndent(withNS, "", "  ")
}

/* ------------ Request types (same as earlier design, trimmed) ------------ */

/* ------------ Minimal FHIR XML struct model ------------- */

const fhirNS = "http://hl7.org/fhir"

type Attr struct {
	XMLName xml.Name `xml:""`
	Value   string   `xml:"value,attr"`
}
type Text struct {
	XMLName xml.Name `xml:""`
	Value   string   `xml:"value,attr"`
}

type Meta struct {
	XMLName     xml.Name `xml:"meta"`
	LastUpdated Attr     `xml:"lastUpdated"`
	Profile     Attr     `xml:"profile"`
}

type Identifier struct {
	XMLName xml.Name `xml:"identifier"`
	System  Attr     `xml:"system"`
	Value   Attr     `xml:"value"`
}

type Coding struct {
	XMLName xml.Name `xml:"coding"`
	System  Attr     `xml:"system"`
	Code    Attr     `xml:"code"`
	Display *Attr    `xml:"display,omitempty"`
}
type CodeableConcept struct {
	XMLName xml.Name `xml:""`
	Coding  []Coding `xml:"coding"`
	Text    *Text    `xml:"text,omitempty"`
}

type Reference struct {
	XMLName  xml.Name `xml:"reference"`
	RefValue string   `xml:"value,attr"`
}

/* ---- Bundle & Entry ---- */

type Entry struct {
	XMLName  xml.Name      `xml:"entry"`
	FullURL  string        `xml:"fullUrl"`
	Resource EntryResource `xml:"resource"`
}

type EntryResource struct {
	// Exactly one of the pointers below should be non-nil.
	MessageHeader      *MessageHeader      `xml:"MessageHeader,omitempty"`
	Organization       *Organization       `xml:"Organization,omitempty"`
	DocumentBundle     *Bundle             `xml:"Bundle,omitempty"` // the inner document bundle
	Practitioner       *Practitioner       `xml:"Practitioner,omitempty"`
	PractitionerRole   *PractitionerRole   `xml:"PractitionerRole,omitempty"`
	Patient            *PatientXML         `xml:"Patient,omitempty"`
	Encounter          *EncounterXML       `xml:"Encounter,omitempty"`
	Observation        *Observation        `xml:"Observation,omitempty"`
	ClinicalImpression *ClinicalImpression `xml:"ClinicalImpression,omitempty"`
	Composition        *Composition        `xml:"Composition,omitempty"`
	MedicationDispense *MedicationDispense `xml:"MedicationDispense,omitempty"`
	// ...add other resource types you emit
}

type Bundle struct {
	XMLName    xml.Name   `xml:"Bundle"`
	ID         Attr       `xml:"id"`
	Meta       Meta       `xml:"meta"`
	Identifier Identifier `xml:"identifier"`
	Type       Text       `xml:"type"`
	Entry      []Entry    `xml:"entry"`
}

/* ---- MessageHeader ---- */

type MessageHeader struct {
	XMLName   xml.Name           `xml:"MessageHeader"`
	ID        Attr               `xml:"id"`
	Meta      Meta               `xml:"meta"`
	Extension []MHOuterExtension `xml:"extension"`
	Event     CodingEvent        `xml:"event"`
	Sender    struct {
		Reference Reference `xml:"reference"`
	} `xml:"sender"`
	Timestamp Text `xml:"timestamp"`
	Source    struct {
		Endpoint Attr `xml:"endpoint"`
	} `xml:"source"`
	Focus struct {
		Reference Reference `xml:"reference"`
	} `xml:"focus"`
}
type MHOuterExtension struct {
	XMLName   xml.Name         `xml:"extension"`
	URL       string           `xml:"url,attr"`
	Extension []MHSubExtension `xml:"extension,omitempty"`
}
type ValueReference struct {
	XMLName   xml.Name  `xml:"valueReference"`
	Reference Reference `xml:"reference"`
}

type ValueCoding struct {
	XMLName xml.Name `xml:"valueCoding"`
	System  Attr     `xml:"system"`
	Code    Attr     `xml:"code"`
	Display *Attr    `xml:"display,omitempty"`
}

type MHSubExtension struct {
	XMLName      xml.Name        `xml:"extension"`
	URL          string          `xml:"url,attr"`
	ValueBoolean *bool           `xml:"valueBoolean,omitempty"`
	ValueCoding  *ValueCoding    `xml:"valueCoding,omitempty"`
	ValueRef     *ValueReference `xml:"valueReference,omitempty"`
	ValueString  *Text           `xml:"valueString,omitempty"`
}
type CodingEvent struct {
	XMLName xml.Name `xml:"event"`
	System  Attr     `xml:"system"`
	Code    Attr     `xml:"code"`
	Display Attr     `xml:"display"`
}

/* ---- Organization ---- */

type Organization struct {
	XMLName    xml.Name     `xml:"Organization"`
	ID         Attr         `xml:"id"`
	Meta       Meta         `xml:"meta"`
	Identifier []Identifier `xml:"identifier"`
	Name       *Text        `xml:"name,omitempty"`
}

/* ---- Patient ---- */

type PatientXML struct {
	XMLName    xml.Name     `xml:"Patient"`
	ID         Attr         `xml:"id"`
	Meta       Meta         `xml:"meta"`
	Identifier []Identifier `xml:"identifier"`
	Name       []HumanName  `xml:"name"`
	Gender     *Text        `xml:"gender,omitempty"`
	BirthDate  Text         `xml:"birthDate"`
	Address    []Address    `xml:"address,omitempty"`
}
type HumanName struct {
	XMLName xml.Name `xml:"name"`
	Use     *Text    `xml:"use,omitempty"`
	Family  Text     `xml:"family"`
	Given   []Text   `xml:"given,omitempty"`
	Prefix  []Text   `xml:"prefix,omitempty"`
}
type Address struct {
	XMLName    xml.Name `xml:"address"`
	PostalCode *Text    `xml:"postalCode,omitempty"`
}

/* ---- Practitioner & PractitionerRole ---- */

type Practitioner struct {
	XMLName    xml.Name     `xml:"Practitioner"`
	ID         Attr         `xml:"id"`
	Meta       Meta         `xml:"meta"`
	Identifier []Identifier `xml:"identifier,omitempty"`
	Name       []HumanName  `xml:"name,omitempty"`
}
type PractitionerRole struct {
	XMLName      xml.Name `xml:"PractitionerRole"`
	ID           Attr     `xml:"id"`
	Meta         Meta     `xml:"meta"`
	Practitioner struct {
		Reference Reference `xml:"reference"`
	} `xml:"practitioner"`
	Organization struct {
		Reference Reference `xml:"reference"`
	} `xml:"organization"`
	Code CodeableConcept `xml:"code"`
}

/* ---- Encounter ---- */

type EncounterXML struct {
	XMLName    xml.Name             `xml:"Encounter"`
	ID         Attr                 `xml:"id"`
	Meta       Meta                 `xml:"meta"`
	Extension  []EncounterExtension `xml:"extension,omitempty"`
	Identifier []Identifier         `xml:"identifier"`
	Status     Text                 `xml:"status"`
	Type       []CodeableConcept    `xml:"type"`
	Subject    struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Participant     []EncounterParticipant `xml:"participant"`
	Period          *Period                `xml:"period,omitempty"`
	Reason          []CodeableConcept      `xml:"reason,omitempty"`
	ServiceProvider struct {
		Reference Reference `xml:"reference"`
	} `xml:"serviceProvider"`
}
type EncounterExtension struct {
	XMLName xml.Name        `xml:"extension"`
	URL     string          `xml:"url,attr"`
	ValueCC CodeableConcept `xml:"valueCodeableConcept"`
}
type EncounterParticipant struct {
	XMLName    xml.Name          `xml:"participant"`
	Type       []CodeableConcept `xml:"type"`
	Individual struct {
		Reference Reference `xml:"reference"`
	} `xml:"individual"`
}
type Period struct {
	XMLName xml.Name `xml:"period"`
	Start   Text     `xml:"start"`
}

/* ---- Composition ---- */

type Composition struct {
	XMLName    xml.Name        `xml:"Composition"`
	ID         Attr            `xml:"id"`
	Meta       Meta            `xml:"meta"`
	Identifier Identifier      `xml:"identifier"`
	Status     Text            `xml:"status"`
	Type       CodeableConcept `xml:"type"`
	Subject    struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Encounter struct {
		Reference Reference `xml:"reference"`
	} `xml:"encounter"`
	Date   Text `xml:"date"`
	Author []struct {
		Reference Reference `xml:"reference"`
	} `xml:"author"`
	Title   Text `xml:"title"`
	Section []struct {
		XMLName xml.Name `xml:"section"`
		Entry   []struct {
			Reference Reference `xml:"reference"`
		} `xml:"entry"`
	} `xml:"section"`
}

/* ---- Observation ---- */

type Observation struct {
	XMLName    xml.Name          `xml:"Observation"`
	ID         Attr              `xml:"id"`
	Meta       Meta              `xml:"meta"`
	Identifier []Identifier      `xml:"identifier"`
	Status     Text              `xml:"status"`
	Category   []CodeableConcept `xml:"category,omitempty"`
	Code       CodeableConcept   `xml:"code"`
	Subject    struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Context struct {
		Reference Reference `xml:"reference"`
	} `xml:"context"`
	EffectiveDateTime *Text `xml:"effectiveDateTime,omitempty"`
	Issued            *Text `xml:"issued,omitempty"`
	Performer         []struct {
		Reference Reference `xml:"reference"`
	} `xml:"performer"`
	BodySite             *CodeableConcept          `xml:"bodySite,omitempty"`
	ValueQuantity        *ValueQuantity            `xml:"valueQuantity,omitempty"`
	ValueCodeableConcept *CodeableConcept          `xml:"valueCodeableConcept,omitempty"`
	Component            []ObservationComponentXML `xml:"component,omitempty"`
}
type ValueQuantity struct {
	XMLName xml.Name `xml:"valueQuantity"`
	Value   *Text    `xml:"value,omitempty"`
	Unit    *Text    `xml:"unit,omitempty"`
	System  *Text    `xml:"system,omitempty"`
	Code    *Text    `xml:"code,omitempty"`
}

type ObservationComponentXML struct {
	XMLName              xml.Name         `xml:"component"`
	Code                 CodeableConcept  `xml:"code"`
	ValueQuantity        *ValueQuantity   `xml:"valueQuantity,omitempty"`
	ValueCodeableConcept *CodeableConcept `xml:"valueCodeableConcept,omitempty"`
}

/* ---- ClinicalImpression ---- */

type ClinicalImpression struct {
	XMLName    xml.Name     `xml:"ClinicalImpression"`
	ID         Attr         `xml:"id"`
	Meta       Meta         `xml:"meta"`
	Identifier []Identifier `xml:"identifier"`
	Status     Text         `xml:"status"`
	Subject    struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Context struct {
		Reference Reference `xml:"reference"`
	} `xml:"context"`
	Date     Text `xml:"date"`
	Assessor struct {
		Reference Reference `xml:"reference"`
	} `xml:"assessor"`
	Summary Text `xml:"summary"`
}

/* ---- DocumentReference (simple) ---- */

type DocumentReference struct {
	XMLName xml.Name `xml:"DocumentReference"`
	ID      Attr     `xml:"id"`
	Meta    Meta     `xml:"meta"`
	Status  Text     `xml:"status"`
	Subject struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Content []struct {
		XMLName    xml.Name `xml:"content"`
		Attachment struct {
			XMLName     xml.Name `xml:"attachment"`
			ContentType Text     `xml:"contentType"`
			Title       *Text    `xml:"title,omitempty"`
			Data        Text     `xml:"data"`
		} `xml:"attachment"`
	} `xml:"content"`
	Description *Text `xml:"description,omitempty"`
}

// MedicationDispense represents a FHIR STU3 MedicationDispense resource
// using the same XML conventions as your other resources.
type MedicationDispense struct {
	XMLName                   xml.Name         `xml:"MedicationDispense"`
	ID                        Attr             `xml:"id"`
	Meta                      Meta             `xml:"meta"`
	Identifier                Identifier       `xml:"identifier"`
	Status                    Text             `xml:"status"` // completed | in-progress etc.
	Category                  *CodeableConcept `xml:"category,omitempty"`
	MedicationCodeableConcept *CodeableConcept `xml:"medicationCodeableConcept"`
	Subject                   *struct {
		Reference Reference `xml:"reference"`
	} `xml:"subject"`
	Context *struct {
		Reference Reference `xml:"reference"`
	} `xml:"context,omitempty"`
	Performer         []MedicationDispensePerformer `xml:"performer,omitempty"`
	Type              *CodeableConcept              `xml:"type,omitempty"`
	Quantity          *QuantityXML                  `xml:"quantity,omitempty"`
	DaysSupply        *DaysSupplyXML                `xml:"daysSupply,omitempty"`
	WhenPrepared      *Text                         `xml:"whenPrepared,omitempty"`
	WhenHandedOver    *Text                         `xml:"whenHandedOver,omitempty"`
	DosageInstruction []MedicationDosageInstruction `xml:"dosageInstruction,omitempty"`
}

// Generic quantity: element name comes from the field tag (e.g. xml:"numerator")
type Quantity struct {
	Value  *Text `xml:"value,omitempty"`
	Unit   *Text `xml:"unit,omitempty"`
	System *Attr `xml:"system,omitempty"`
	Code   *Attr `xml:"code,omitempty"`
}

// <quantity> ... </quantity>
type QuantityXML struct {
	XMLName xml.Name `xml:"quantity"`
	Value   *Text    `xml:"value,omitempty"`
	Unit    *Text    `xml:"unit,omitempty"`
	System  *Attr    `xml:"system,omitempty"`
	Code    *Attr    `xml:"code,omitempty"`
}

// <daysSupply> ... </daysSupply>
type DaysSupplyXML struct {
	XMLName xml.Name `xml:"daysSupply"`
	Value   *Text    `xml:"value,omitempty"`
	Unit    *Text    `xml:"unit,omitempty"`
	System  *Attr    `xml:"system,omitempty"`
	Code    *Attr    `xml:"code,omitempty"`
}

// MedicationDispensePerformer captures who supplied the medication.
type MedicationDispensePerformer struct {
	Actor *struct {
		Reference Reference `xml:"reference"`
	} `xml:"actor"`
}

// MedicationDosageInstruction mirrors the FHIR dosageInstruction backbone element.
type MedicationDosageInstruction struct {
	Text               *Text            `xml:"text,omitempty"`
	PatientInstruction *Text            `xml:"patientInstruction,omitempty"`
	Timing             *Timing          `xml:"timing,omitempty"`
	Route              *CodeableConcept `xml:"route,omitempty"`
	MaxDosePerPeriod   *Ratio           `xml:"maxDosePerPeriod,omitempty"`
}

// Timing and its nested repeat follow the same style you already have.
type Timing struct {
	Repeat *TimingRepeat `xml:"repeat,omitempty"`
}

type TimingRepeat struct {
	Frequency  *int    `xml:"frequency,omitempty"`
	Period     float32 `xml:"period,omitempty"`
	PeriodUnit *Text   `xml:"periodUnit,omitempty"`
}

// Ratio is used for maxDosePerPeriod
type Ratio struct {
	Numerator   *Quantity `xml:"numerator,omitempty"`
	Denominator *Quantity `xml:"denominator,omitempty"`
}

/* ------------ builders ------------- */

func makeMessageHeader(id, docBundleID, orgID string, cfg Config, opts *http.MessageHeaderOptions, lastUpdated string) MessageHeader {
	bus := cfg.DefaultBusinessAckRequested
	inf := cfg.DefaultInfrastructureAckRequested
	rec := cfg.DefaultRecipientType
	if opts != nil {
		if opts.BusinessAckRequested != nil {
			bus = *opts.BusinessAckRequested
		}
		if opts.InfrastructureAckRequested != nil {
			inf = *opts.InfrastructureAckRequested
		}
		if opts.RecipientType != nil {
			rec = *opts.RecipientType
		}
	}

	var display *string
	if val, ok := map[string]string{"FI": "For Information"}[rec]; ok {
		display = &val
	}

	ext := MHOuterExtension{
		URL: "https://fhir.nhs.uk/STU3/StructureDefinition/Extension-ITK-MessageHandling-2",
		Extension: []MHSubExtension{
			{URL: "BusAckRequested", ValueBoolean: &bus},
			{URL: "InfAckRequested", ValueBoolean: &inf},
			{
				URL: "RecipientType",
				ValueCoding: &ValueCoding{
					System:  Attr{Value: "https://fhir.nhs.uk/STU3/CodeSystem/ITK-RecipientType-1"},
					Code:    Attr{Value: rec},
					Display: optAttr(display),
				},
			},
		},
	}
	if opts != nil && opts.MessageDefinitionRef != nil {
		ext.Extension = append(ext.Extension, MHSubExtension{
			URL:      "MessageDefinition",
			ValueRef: &ValueReference{Reference: Reference{RefValue: *opts.MessageDefinitionRef}},
		})
	}
	if opts != nil && opts.SenderReference != nil {
		ext.Extension = append(ext.Extension, MHSubExtension{
			URL: "SenderReference", ValueString: &Text{Value: *opts.SenderReference},
		})
	}
	if opts != nil && opts.LocalExtension != nil {
		ext.Extension = append(ext.Extension, MHSubExtension{
			URL: "LocalExtension", ValueString: &Text{Value: *opts.LocalExtension},
		})
	} else {
		ext.Extension = append(ext.Extension, MHSubExtension{
			URL: "LocalExtension", ValueString: &Text{Value: "None"},
		})
	}

	h := MessageHeader{
		ID:        Attr{Value: trimURN(id)},
		Meta:      Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/ITK-MessageHeader-2"}},
		Extension: []MHOuterExtension{ext},
		Event: CodingEvent{
			System:  Attr{Value: "https://fhir.nhs.uk/STU3/CodeSystem/ITK-MessageEvent-2"},
			Code:    Attr{Value: "ITK014M"},
			Display: Attr{Value: "ITK Update Record"},
		},
		Timestamp: Text{Value: time.Now().Format(time.RFC3339Nano)},
	}
	h.Sender.Reference = Reference{RefValue: idRef(orgID)}
	h.Source.Endpoint = Attr{Value: cfg.SenderMeshMailbox}
	h.Focus.Reference = Reference{RefValue: idRef(docBundleID)}
	return h
}

func makeOrganization(id, profile, ods, lastUpdated string) Organization {
	return Organization{
		ID:   Attr{Value: trimURN(id)},
		Meta: Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: profile}},
		Identifier: []Identifier{
			{System: Attr{Value: "https://fhir.nhs.uk/Id/ods-organization-code"}, Value: Attr{Value: ods}},
		},
	}
}

func makePatient(id string, p http.Patient, lastUpdated string) PatientXML {
	ids := []Identifier{
		{System: Attr{Value: "https://fhir.nhs.uk/Id/nhs-number"}, Value: Attr{Value: p.NhsNumber}},
	}
	// NHS number verification extension (simplified; attached as separate identifier extension is omitted)
	name := HumanName{
		Use:    &Text{Value: "official"},
		Family: Text{Value: p.Surname},
	}
	if p.GivenName != nil && strings.TrimSpace(*p.GivenName) != "" {
		name.Given = []Text{{Value: *p.GivenName}}
	}
	addr := []Address{}
	if p.Postcode != nil {
		addr = []Address{{PostalCode: &Text{Value: *p.Postcode}}}
	}
	var gender *Text
	if p.Gender != nil {
		gender = &Text{Value: string(*p.Gender)}
	}
	return PatientXML{
		ID:         Attr{Value: trimURN(id)},
		Meta:       Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Patient-1"}},
		Identifier: ids,
		Name:       []HumanName{name},
		Gender:     gender,
		BirthDate:  Text{Value: p.DateOfBirth.String()},
		Address:    addr,
	}
}

func makePractitioner(id string, a http.Author, lastUpdated string) Practitioner {
	ids := []Identifier{}
	if a.Identifiers != nil {
		for _, i := range *a.Identifiers {
			ids = append(ids, Identifier{System: Attr{Value: i.System}, Value: Attr{Value: i.Value}})
		}
	}

	if a.ProfessionalCode != nil {
		ids = append(ids, Identifier{System: Attr{Value: "https://fhir.provider.example/identifier/staff-code"}, Value: Attr{Value: *a.ProfessionalCode}})
	}
	prefix, given, family := splitName(a.Name)
	name := HumanName{Family: Text{Value: family}}
	if given != "" {
		name.Given = []Text{{Value: given}}
	}
	if prefix != "" {
		name.Prefix = []Text{{Value: prefix}}
	}
	return Practitioner{
		ID:         Attr{Value: trimURN(id)},
		Meta:       Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Practitioner-1"}},
		Identifier: ids,
		Name:       []HumanName{name},
	}
}

func makePractitionerRole(id, prID, orgID string, role http.CodeableConcept, lastUpdated string) PractitionerRole {
	return PractitionerRole{
		ID:   Attr{Value: trimURN(id)},
		Meta: Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-PractitionerRole-1"}},
		Practitioner: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(prID)}},
		Organization: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(orgID)}},
		Code: CodeableConcept{
			Coding: []Coding{{System: Attr{Value: role.System}, Code: Attr{Value: role.Code} /*Display: optText(*role.Display)*/}},
			Text:   optText(*role.Display),
		},
	}
}

func makeEncounter(id string, e http.Encounter, patientID, practitionerID, orgID, lastUpdated string) EncounterXML {
	out := EncounterXML{
		ID:         Attr{Value: trimURN(id)},
		Meta:       Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Encounter-1"}},
		Identifier: []Identifier{{System: Attr{Value: "https://fhir.provider.example/identifier/encounter"}, Value: Attr{Value: trimURN(id)}}},
		Status:     Text{Value: "finished"},
		Type: []CodeableConcept{{
			Coding: []Coding{{System: Attr{Value: "http://snomed.info/sct"}, Code: Attr{Value: "307778003"} /*Display: optText("Seen in primary care establishment")*/}},
			Text:   optText("Seen in primary care establishment"),
		}},
		Subject: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(patientID)}},
		Participant: []EncounterParticipant{{
			Type: []CodeableConcept{{
				Coding: []Coding{{System: Attr{Value: "https://fhir.nhs.uk/STU3/CodeSystem/GPConnect-ParticipantType-1"}, Code: Attr{Value: "REC"}}},
				Text:   optText("recorder"),
			}},
			Individual: struct {
				Reference Reference `xml:"reference"`
			}{Reference: Reference{RefValue: idRef(practitionerID)}},
		}},
		ServiceProvider: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(orgID)}},
	}
	if e.OccurredAt != nil {
		out.Period = &Period{Start: Text{Value: e.OccurredAt.Format("2006-01-02")}}
	}
	if e.ReasonCode != nil && e.ReasonCode.System != "" && e.ReasonCode.Code != "" {
		out.Reason = []CodeableConcept{{
			Coding: []Coding{{System: Attr{Value: e.ReasonCode.System}, Code: Attr{Value: e.ReasonCode.Code} /*Display: optText(e.ReasonCode.Display)*/}},
			Text:   optText(*e.ReasonCode.Display),
		}}
	} else if e.Reason != nil {
		out.Reason = []CodeableConcept{{
			Text: optText(*e.Reason),
		}}
	}
	if e.OutcomeOfAttendance != nil && e.OutcomeOfAttendance.System != "" && e.OutcomeOfAttendance.Code != "" {
		if e.OutcomeOfAttendance.Display != nil {
			out.Extension = []EncounterExtension{{
				URL: "https://fhir.hl7.org.uk/STU3/StructureDefinition/Extension-CareConnect-OutcomeOfAttendance-1",
				ValueCC: CodeableConcept{
					Coding: []Coding{{System: Attr{Value: e.OutcomeOfAttendance.System}, Code: Attr{Value: e.OutcomeOfAttendance.Code} /*Display: optText(e.OutcomeOfAttendance.Display)*/}},
					Text:   optText(*e.OutcomeOfAttendance.Display),
				},
			}}
		}
	}
	return out
}

func makeComposition(id string, req http.UpdateRecordRequest, patientID, encounterID, authorID, lastUpdated string) Composition {
	cc := codedOrDefault(req.Composition)
	section := struct {
		XMLName xml.Name `xml:"section"`
		Entry   []struct {
			Reference Reference `xml:"reference"`
		} `xml:"entry"`
	}{}

	refsInOrder := []string{
		"urn:uuid:e9a3673b-12d2-4001-826f-626e30d5f71e",
		"urn:uuid:62db03c2-148b-4872-8d8b-2ea19871c00d",
		"urn:uuid:c11ba114-d1bf-461c-a5ee-10e82df9dcc8",
		"urn:uuid:75c5672f-b2d6-435f-80c8-a4864258162b",
		"urn:uuid:c33b855b-4824-4876-9509-ed34de2109b4",
		"urn:uuid:e7c459a8-2f3f-4968-aa37-937e0a0b88dd",
		"urn:uuid:0843a500-9f0c-4043-bfd8-b03ca0e95735",
		"urn:uuid:cb8e1553-4bf1-474c-ab42-f70219b1545a",
		"urn:uuid:3ecca922-9841-43b6-8389-edc4936228dd",
	}
	for _, r := range refsInOrder {
		section.Entry = append(section.Entry, struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: r}})
	}

	// We’ll include references to all entries present later by consumers—here we leave it empty (optional), or you can populate if you collected refs.
	return Composition{
		ID:         Attr{Value: trimURN(id)},
		Meta:       Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.hl7.org.uk/STU3/StructureDefinition/CareConnect-Composition-1"}},
		Identifier: Identifier{System: Attr{Value: "https://fhir.provider.example/identifier/composition"}, Value: Attr{Value: trimURN(id)}},
		Status:     Text{Value: "final"},
		Type:       CodeableConcept{Coding: []Coding{{System: Attr{Value: cc.System}, Code: Attr{Value: cc.Code}, Display: optAttr(cc.Display)}}, Text: optText(*cc.Display)},
		Subject: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(patientID)}},
		Encounter: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(encounterID)}},
		Date: Text{Value: time.Now().Format("2006-01-02")},
		Author: []struct {
			Reference Reference `xml:"reference"`
		}{{Reference: Reference{RefValue: idRef(authorID)}}},
		Title: Text{Value: defaultString(req.Composition != nil && *req.Composition.Title != "", *req.Composition.Title, "Community service update")},
		Section: []struct {
			XMLName xml.Name `xml:"section"`
			Entry   []struct {
				Reference Reference `xml:"reference"`
			} `xml:"entry"`
		}{section},
	}
}

func defaultString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func makeObservation(id string, ob http.ObservationInput, patientID, encounterID, performerID, lastUpdated string) Observation {
	obs := Observation{
		ID:         Attr{Value: trimURN(id)},
		Meta:       Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-Observation-1"}},
		Identifier: []Identifier{{System: Attr{Value: "https://fhir.provider.example/identifier/observation"}, Value: Attr{Value: trimURN(id)}}},
		Status:     Text{Value: "final"},
		Code:       CodeableConcept{Coding: []Coding{{System: Attr{Value: ob.Code.Code}, Code: Attr{Value: ob.Code.Code}, Display: optAttr(ob.Code.Display)}}, Text: optText(*ob.Code.Text)},
		Subject: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(patientID)}},
		Context: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(encounterID)}},
		Performer: []struct {
			Reference Reference `xml:"reference"`
		}{{Reference: Reference{RefValue: idRef(performerID)}}},
	}
	// category
	if ob.Category != nil && ob.Category.System != "" && ob.Category.Code != "" {
		obs.Category = []CodeableConcept{{
			Coding: []Coding{{System: Attr{Value: ob.Category.System}, Code: Attr{Value: ob.Category.Code}, Display: optAttr(ob.Category.Display)}},
			Text:   optText(*ob.Category.Display),
		}}
	} else if ob.Category != nil {
		obs.Category = []CodeableConcept{{
			Coding: []Coding{{System: Attr{Value: "http://terminology.hl7.org/CodeSystem/observation-category"}, Code: Attr{Value: ob.Category.Code}}},
			Text:   optText(*ob.Category.Text),
		}}
	}
	// timing
	if !ob.EffectiveDateTime.IsZero() {
		obs.EffectiveDateTime = &Text{Value: ob.EffectiveDateTime.Format("2006-01-02")}
	}
	if ob.Issued != nil {
		obs.Issued = &Text{Value: ob.Issued.Format(time.RFC3339Nano)}
	}
	// bodySite
	if ob.BodySite != nil && ob.BodySite.System != "" && ob.BodySite.Code != "" {
		obs.BodySite = &CodeableConcept{Coding: []Coding{{System: Attr{Value: ob.BodySite.System}, Code: Attr{Value: ob.BodySite.Code} /*Display: optText(ob.BodySite.Display)*/}}, Text: optText(*ob.BodySite.Display)}
	}
	// values
	// todo
	/*switch {
	case ob.ValueQuantity != nil:
		obs.ValueQuantity = qToXML(*ob.ValueQuantity)
	case ob.ValueCodeableConcept != nil:
		obs.ValueCodeableConcept = &CodeableConcept{Coding: []Coding{{System: Attr{Value: ob.ValueCodeableConcept.System}, Code: Attr{Value: ob.ValueCodeableConcept.Code} /*Display: optText(ob.ValueCodeableConcept.Display)}}, Text: optText(*ob.ValueCodeableConcept.Display)}*/
	/*case ob.Value != nil:
		obs.ValueQuantity = &ValueQuantity{
			Value: &Text{Value: *ob.Value},
			Unit:  optText(*ob.Unit),
		}
	}*/
	// components
	if ob.Components != nil {
		for _, c := range *ob.Components {
			comp := ObservationComponentXML{
				Code: CodeableConcept{Coding: []Coding{{System: Attr{Value: c.Code.System}, Code: Attr{Value: c.Code.Code} /*Display: optText(c.Code.Display)*/}}, Text: optText(*c.Code.Display)},
			}
			if c.ValueQuantity != nil {
				comp.ValueQuantity = qToXML(*c.ValueQuantity)
			}
			if c.ValueCodeableConcept != nil {
				comp.ValueCodeableConcept = &CodeableConcept{Coding: []Coding{{System: Attr{Value: c.ValueCodeableConcept.System}, Code: Attr{Value: c.ValueCodeableConcept.Code} /*Display: optText(c.ValueCodeableConcept.Display)*/}}, Text: optText(*c.ValueCodeableConcept.Display)}
			}
			obs.Component = append(obs.Component, comp)
		}
	}

	return obs
}

func makeClinicalImpression(id string, nb http.NarrativeBlock, patientID, encID, assessorID, lastUpdated string) ClinicalImpression {
	meta := Meta{LastUpdated: Attr{Value: lastUpdated}, Profile: Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-ClinicalImpression-1"}}
	return ClinicalImpression{
		ID:         Attr{Value: trimURN(id)},
		Meta:       meta,
		Identifier: []Identifier{{System: Attr{Value: "https://fhir.provider.example/identifier/clinical-impression"}, Value: Attr{Value: trimURN(id)}}},
		Status:     Text{Value: "completed"},
		Subject: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(patientID)}},
		Context: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(encID)}},
		Date: Text{Value: time.Now().Format("2006-01-02")},
		Assessor: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(assessorID)}},
		Summary: Text{Value: nb.Text},
	}
}

func makeDocumentReference(id string, att http.Attachment, patientID, lastUpdated string) DocumentReference {
	dr := DocumentReference{
		ID:     Attr{Value: trimURN(id)},
		Meta:   Meta{LastUpdated: Attr{Value: lastUpdated}},
		Status: Text{Value: "current"},
		Subject: struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(patientID)}},
		Description: optText(*att.Description),
	}
	var c struct {
		XMLName    xml.Name `xml:"content"`
		Attachment struct {
			XMLName     xml.Name `xml:"attachment"`
			ContentType Text     `xml:"contentType"`
			Title       *Text    `xml:"title,omitempty"`
			Data        Text     `xml:"data"`
		} `xml:"attachment"`
	}
	c.Attachment.ContentType = Text{Value: att.ContentType}
	if att.Title != nil {
		c.Attachment.Title = &Text{Value: *att.Title}
	}
	c.Attachment.Data = Text{Value: att.Base64}
	dr.Content = []struct {
		XMLName    xml.Name `xml:"content"`
		Attachment struct {
			XMLName     xml.Name `xml:"attachment"`
			ContentType Text     `xml:"contentType"`
			Title       *Text    `xml:"title,omitempty"`
			Data        Text     `xml:"data"`
		} `xml:"attachment"`
	}{c}
	return dr
}

// makeMedicationDispense builds a MedicationDispense entry and returns the entry plus its urn:uuid
func makeMedicationDispense(
	ms http.MedicationSupplied,
	patientID, encounterID, authorID string,
	lastUpdated string,
) (Entry, string) {

	mdID := newURN() // your helper returning "urn:uuid:...."
	res := MedicationDispense{
		ID: Attr{Value: trimURN(mdID)},
		Meta: Meta{
			LastUpdated: Attr{Value: lastUpdated},
			Profile:     Attr{Value: "https://fhir.nhs.uk/STU3/StructureDefinition/CareConnect-GPC-MedicationDispense-1"},
		},
		Identifier: Identifier{
			System: Attr{Value: "https://fhir.provider.example/identifier/medication-dispense"},
			Value:  Attr{Value: trimURN(mdID)},
		},
		//Status: Text{Value: defaultString(ms.Status, "completed")},
	}

	// Category (optional)
	if ms.Category != nil {
		mdCat := CodeableConcept{
			Coding: []Coding{{
				System:  Attr{Value: ms.Category.System},
				Code:    Attr{Value: ms.Category.Code},
				Display: optAttr(ms.Category.Display),
			}},
			Text: optText(*ms.Category.Display),
		}
		res.Category = &mdCat
	}

	// MedicationCodeableConcept (required)
	res.MedicationCodeableConcept = &CodeableConcept{
		Coding: []Coding{{
			System:  Attr{Value: ms.Medication.System},
			Code:    Attr{Value: ms.Medication.Code},
			Display: optAttr(ms.Medication.Display),
		}},
		Text: optText(*ms.Medication.Display),
	}

	// Subject / Context / Performer
	res.Subject = &struct {
		Reference Reference `xml:"reference"`
	}{Reference: Reference{RefValue: idRef(patientID)}}
	res.Context = &struct {
		Reference Reference `xml:"reference"`
	}{Reference: Reference{RefValue: idRef(encounterID)}}
	res.Performer = []MedicationDispensePerformer{{
		Actor: &struct {
			Reference Reference `xml:"reference"`
		}{Reference: Reference{RefValue: idRef(authorID)}},
	}}

	// Supply type (optional)
	if ms.SupplyType != nil {
		res.Type = &CodeableConcept{
			Coding: []Coding{{
				System:  Attr{Value: ms.SupplyType.System},
				Code:    Attr{Value: ms.SupplyType.Code},
				Display: optAttr(ms.SupplyType.Display),
			}},
			Text: optText(*ms.SupplyType.Display),
		}
	}

	// Quantity / DaysSupply (optional)
	if ms.Quantity != nil {
		res.Quantity = &QuantityXML{
			Value:  optText(fmt.Sprintf("%.0f", ms.Quantity.Value)),
			Unit:   optText(*ms.Quantity.Unit),
			System: optAttr(ms.Quantity.System),
			Code:   optAttr(ms.Quantity.Code),
		}
	}
	if ms.DaysSupply != nil {
		res.DaysSupply = &DaysSupplyXML{
			Value:  optText(fmt.Sprintf("%.0f", ms.DaysSupply.Value)),
			Unit:   optText(*ms.DaysSupply.Unit),
			System: optAttr(ms.DaysSupply.System),
			Code:   optAttr(ms.DaysSupply.Code),
		}
	}

	// When prepared / handed over
	if ms.WhenPrepared != nil {
		res.WhenPrepared = optText(ms.WhenPrepared.String())
	}
	if ms.WhenHandedOver != nil {
		res.WhenHandedOver = optText(ms.WhenHandedOver.String())
	}

	// DosageInstruction (optional)
	if ms.DosageInstruction != nil {
		di := MedicationDosageInstruction{
			Text:               optText(*ms.DosageInstruction.Text),
			PatientInstruction: optText(*ms.DosageInstruction.PatientInstruction),
		}
		if ms.DosageInstruction.Timing != nil {
			di.Timing = &Timing{
				Repeat: &TimingRepeat{
					Frequency:  ms.DosageInstruction.Timing.Frequency,
					Period:     *ms.DosageInstruction.Timing.Period,
					PeriodUnit: optText(*ms.DosageInstruction.Timing.PeriodUnit),
				},
			}
		}
		if ms.DosageInstruction.Route != nil {
			di.Route = &CodeableConcept{
				Coding: []Coding{{
					System:  Attr{Value: ms.DosageInstruction.Route.System},
					Code:    Attr{Value: ms.DosageInstruction.Route.Code},
					Display: optAttr(ms.DosageInstruction.Route.Display),
				}},
				Text: optText(*ms.DosageInstruction.Route.Display),
			}
		}
		if mdp := ms.DosageInstruction.MaxDosePerPeriod; mdp != nil {
			di.MaxDosePerPeriod = &Ratio{
				Numerator: &Quantity{
					Value:  optDecimal(&mdp.Numerator.Value),
					Unit:   optText(*mdp.Numerator.Unit),
					System: optAttr(mdp.Numerator.System),
					Code:   optAttr(mdp.Numerator.Code),
				},
				Denominator: &Quantity{
					Value:  optDecimal(&mdp.Denominator.Value),
					Unit:   optText(*mdp.Denominator.Unit),
					System: optAttr(mdp.Denominator.System),
					Code:   optAttr(mdp.Denominator.Code),
				},
			}
		}
		res.DosageInstruction = []MedicationDosageInstruction{di}
	}

	entry := Entry{
		FullURL:  mdID,
		Resource: EntryResource{MedicationDispense: &res},
	}
	return entry, mdID
}

/* ------------ utils ------------- */

func validateMinimal(req http.UpdateRecordRequest) error {
	if strings.TrimSpace(req.Patient.NhsNumber) == "" ||
		strings.TrimSpace(req.Patient.DateOfBirth.String()) == "" ||
		strings.TrimSpace(req.Patient.Surname) == "" {
		return errors.New("patient.nhsNumber, patient.dateOfBirth, patient.surname are required")
	}
	if strings.TrimSpace(req.ClinicalSummary.FreeText) == "" {
		return errors.New("clinicalSummary.freeText is required")
	}
	if strings.TrimSpace(req.Provenance.Author.Name) == "" ||
		req.Provenance.System.Asid == nil ||
		strings.TrimSpace(req.Provenance.System.Name) == "" {
		return errors.New("provenance.author.name and provenance.system.{asid,name} are required")
	}
	if strings.TrimSpace(req.Routing.RegisteredPracticeODS) == "" {
		return errors.New("routing.registeredPracticeODS is required")
	}
	return nil
}

func toEncounter(in http.EncounterWithRole) (http.Encounter, error) {
	var out http.Encounter
	b, _ := json.Marshal(in)
	return out, json.Unmarshal(b, &out)
}

func resolveEncounters(req http.UpdateRecordRequest) (primary http.Encounter, related []http.Encounter) {
	if req.Encounters != nil && len(*req.Encounters) > 0 {
		encounters := *req.Encounters

		first := 0
		for i, e := range encounters {
			if *e.Role == "primary" {
				first = i
				break
			}
		}

		p, _ := toEncounter(encounters[first])

		var r []http.Encounter
		for i, e := range encounters {
			if i != first {
				ee, _ := toEncounter(e)
				r = append(r, ee)
			}
		}
		return p, r
	}

	if req.Encounter != nil {
		return *req.Encounter, nil
	}

	return http.Encounter{}, nil
}

func newURN() string          { return "urn:uuid:" + uuid.New().String() }
func trimURN(u string) string { return strings.TrimPrefix(u, "urn:uuid:") }
func idRef(u string) string   { return u } // keep urn as-is for reference

func optText(s string) *Text {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &Text{Value: s}
}

func optDecimal(f *float32) *Text {
	if f == nil {
		return nil
	}
	return &Text{Value: fmt.Sprintf("%.0f", *f)}
}

func optAttr(s *string) *Attr {
	if s == nil || *s == "" {
		return nil
	}
	return &Attr{Value: *s}
}

func splitName(full string) (prefix, given, family string) {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return
	}
	common := map[string]bool{"Dr": true, "Mr": true, "Mrs": true, "Miss": true, "Ms": true}
	if len(parts) > 1 && common[parts[0]] {
		prefix = parts[0]
		parts = parts[1:]
	}
	if len(parts) == 1 {
		family = parts[0]
		return
	}
	given = strings.Join(parts[:len(parts)-1], " ")
	family = parts[len(parts)-1]
	return
}

func codedOrDefault(cd *http.CompositionDetails) http.CodedItem {
	if cd != nil && cd.Type != nil && cd.Type.System != "" && cd.Type.Code != "" {
		return *cd.Type
	}
	return http.CodedItem{
		System: "http://snomed.info/sct",
		Code:   "1659111000000107",
		//Display: "Community Pharmacy Service",
	}
}

func qToXML(q http.Quantity) *ValueQuantity {
	out := &ValueQuantity{}
	if q.Value != 0 {
		out.Value = &Text{Value: fmt.Sprintf("%g", q.Value)}
	}
	if q.Unit != nil {
		out.Unit = &Text{Value: *q.Unit}
	}
	if q.System != nil {
		out.System = &Text{Value: *q.System}
	}
	if q.Code != nil {
		out.Code = &Text{Value: *q.Code}
	}
	return out
}

/* ---- namespace wrapper ---- */

// Wrap the top-level with xmlns="http://hl7.org/fhir"
type namespacedBundle struct {
	XMLName xml.Name `xml:"Bundle"`
	XMLNS   string   `xml:"xmlns,attr"`
	Bundle
}

func namespaced(b Bundle) namespacedBundle {
	return namespacedBundle{XMLNS: fhirNS, Bundle: b}
}
