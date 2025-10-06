package model

import "encoding/xml"

type Organization struct {
	XMLName    xml.Name    `xml:"Organization"`
	ID         *Attr       `xml:"id,omitempty"`
	Meta       *Meta       `xml:"meta,omitempty"`
	Identifier *Identifier `xml:"identifier,omitempty"`
	Name       *Attr       `xml:"name,omitempty"`
}

type Practitioner struct {
	XMLName    xml.Name    `xml:"Practitioner"`
	ID         *Attr       `xml:"id,omitempty"`
	Meta       *Meta       `xml:"meta,omitempty"`
	Identifier *Identifier `xml:"identifier,omitempty"`
	Name       *HumanName  `xml:"name,omitempty"`
	Gender     *Attr       `xml:"gender,omitempty"`
}

type PractitionerRole struct {
	XMLName      xml.Name         `xml:"PractitionerRole"`
	ID           *Attr            `xml:"id,omitempty"`
	Meta         *Meta            `xml:"meta,omitempty"`
	Practitioner *Reference       `xml:"practitioner,omitempty"`
	Organization *Reference       `xml:"organization,omitempty"`
	Code         *CodeableConcept `xml:"code,omitempty"`
}

type Patient struct {
	XMLName    xml.Name      `xml:"Patient"`
	ID         *Attr         `xml:"id,omitempty"`
	Meta       *Meta         `xml:"meta,omitempty"`
	Identifier []*Identifier `xml:"identifier,omitempty"`
	Name       *HumanName    `xml:"name,omitempty"`
	Gender     *Attr         `xml:"gender,omitempty"`
	BirthDate  *Attr         `xml:"birthDate,omitempty"`
	// Address/telecom can be added later
}
