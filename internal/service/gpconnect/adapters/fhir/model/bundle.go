package model

import "encoding/xml"

type Bundle struct {
	XMLName    xml.Name      `xml:"Bundle"`
	Xmlns      string        `xml:"xmlns,attr,omitempty"`
	ID         *Attr         `xml:"id,omitempty"`
	Meta       *Meta         `xml:"meta,omitempty"`
	Identifier *Identifier   `xml:"identifier,omitempty"`
	Type       *Attr         `xml:"type,omitempty"` // "message" or "document"
	Entry      []BundleEntry `xml:"entry,omitempty"`
}

type BundleEntry struct {
	XMLName  xml.Name      `xml:"entry"`
	FullURL  *Attr         `xml:"fullUrl,omitempty"`
	Resource EntryResource `xml:"resource"`
}

type EntryResource struct {
	// Exactly one of the below should be non-nil
	MessageHeader      *MessageHeader      `xml:"MessageHeader,omitempty"`
	Organization       *Organization       `xml:"Organization,omitempty"`
	Bundle             *Bundle             `xml:"Bundle,omitempty"`
	Composition        *Composition        `xml:"Composition,omitempty"`
	Encounter          *Encounter          `xml:"Encounter,omitempty"`
	Practitioner       *Practitioner       `xml:"Practitioner,omitempty"`
	PractitionerRole   *PractitionerRole   `xml:"PractitionerRole,omitempty"`
	Patient            *Patient            `xml:"Patient,omitempty"`
	ClinicalImpression *ClinicalImpression `xml:"ClinicalImpression,omitempty"`
	// Observation can be added later if needed
}
