package model

import "encoding/xml"

type Composition struct {
	XMLName    xml.Name            `xml:"Composition"`
	ID         *Attr               `xml:"id,omitempty"`
	Meta       *Meta               `xml:"meta,omitempty"`
	Identifier *Identifier         `xml:"identifier,omitempty"`
	Status     *Attr               `xml:"status,omitempty"` // "final"
	Type       *CodeableConcept    `xml:"type,omitempty"`
	Subject    *Reference          `xml:"subject,omitempty"`
	Encounter  *Reference          `xml:"encounter,omitempty"`
	Date       *Attr               `xml:"date,omitempty"`
	Author     []*Reference        `xml:"author,omitempty"`
	Title      *Attr               `xml:"title,omitempty"`
	Section    *CompositionSection `xml:"section,omitempty"`
}

type CompositionSection struct {
	Entry []*Reference `xml:"entry,omitempty"`
}

type Encounter struct {
	XMLName         xml.Name                `xml:"Encounter"`
	ID              *Attr                   `xml:"id,omitempty"`
	Meta            *Meta                   `xml:"meta,omitempty"`
	Identifier      *Identifier             `xml:"identifier,omitempty"`
	Status          *Attr                   `xml:"status,omitempty"` // finished|unknown
	Type            *CodeableConcept        `xml:"type,omitempty"`
	Subject         *Reference              `xml:"subject,omitempty"`
	Participant     []*EncounterParticipant `xml:"participant,omitempty"`
	Period          *Period                 `xml:"period,omitempty"`
	Reason          *CodeableConcept        `xml:"reason,omitempty"`
	ServiceProvider *Reference              `xml:"serviceProvider,omitempty"`
}

type EncounterParticipant struct {
	Type       *CodeableConcept `xml:"type,omitempty"` // holds REC/PPRF/PART
	Individual *Reference       `xml:"individual,omitempty"`
	Period     *Period          `xml:"period,omitempty"`
}

type ClinicalImpression struct {
	XMLName     xml.Name    `xml:"ClinicalImpression"`
	ID          *Attr       `xml:"id,omitempty"`
	Meta        *Meta       `xml:"meta,omitempty"`
	Identifier  *Identifier `xml:"identifier,omitempty"`
	Status      *Attr       `xml:"status,omitempty"` // in-progress|completed|entered-in-error
	Subject     *Reference  `xml:"subject,omitempty"`
	Context     *Reference  `xml:"context,omitempty"` // Encounter
	Date        *Attr       `xml:"date,omitempty"`
	Assessor    *Reference  `xml:"assessor,omitempty"`
	Summary     *Attr       `xml:"summary,omitempty"`
	Description *Attr       `xml:"description,omitempty"`

	Problem []*CodeableConcept           `xml:"problem,omitempty"`
	Finding []*ClinicalImpressionFinding `xml:"finding,omitempty"`
	// Plan omitted for brevity; add if you want to encode it too
}

type ClinicalImpressionFinding struct {
	Item  *CodeableConcept `xml:"item,omitempty"`
	Basis *Attr            `xml:"basis,omitempty"`
}
