package model

import "encoding/xml"

// Attr is a helper for FHIR's <id value="..."/> style
type Attr struct {
	Value string `xml:"value,attr,omitempty"`
}

type Meta struct {
	XMLName     xml.Name `xml:"meta"`
	LastUpdated *Attr    `xml:"lastUpdated,omitempty"`
	Profile     *Attr    `xml:"profile,omitempty"`
	// Optional meta.tag support if needed later:
	Tag *MetaTag `xml:"tag,omitempty"`
}

type MetaTag struct {
	System  *Attr `xml:"system,omitempty"`
	Code    *Attr `xml:"code,omitempty"`
	Display *Attr `xml:"display,omitempty"`
}

type Identifier struct {
	XMLName xml.Name `xml:"identifier"`
	System  *Attr    `xml:"system,omitempty"`
	Value   *Attr    `xml:"value,omitempty"`
	// Optional: extension(s) if ever needed
}

type Coding struct {
	XMLName xml.Name `xml:"coding"`
	System  *Attr    `xml:"system,omitempty"`
	Code    *Attr    `xml:"code,omitempty"`
	Display *Attr    `xml:"display,omitempty"`
}

type CodingEvent struct {
	XMLName xml.Name `xml:"event"`
	System  *Attr    `xml:"system"`
	Code    *Attr    `xml:"code"`
	Display *Attr    `xml:"display,omitempty"`
}

type CodeableConcept struct {
	XMLName xml.Name `xml:""`
	Coding  *Coding  `xml:"coding,omitempty"`
	Text    *Attr    `xml:"text,omitempty"`
}

type Reference struct {
	XMLName   xml.Name `xml:""`
	Reference *Attr    `xml:"reference,omitempty"`
}

type Period struct {
	XMLName xml.Name `xml:"period"`
	Start   *Attr    `xml:"start,omitempty"`
	End     *Attr    `xml:"end,omitempty"`
}

type HumanName struct {
	XMLName xml.Name `xml:"name"`
	Use     *Attr    `xml:"use,omitempty"`
	Family  *Attr    `xml:"family,omitempty"`
	Given   *Attr    `xml:"given,omitempty"`
	Prefix  *Attr    `xml:"prefix,omitempty"`
	// text (free-form) not commonly used in UK XML payloads; add if needed
}
