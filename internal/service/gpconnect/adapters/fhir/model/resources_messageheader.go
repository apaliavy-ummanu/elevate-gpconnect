package model

import "encoding/xml"

type MessageHeader struct {
	XMLName xml.Name `xml:"MessageHeader"`
	ID      *Attr    `xml:"id,omitempty"`
	Meta    *Meta    `xml:"meta,omitempty"`

	// ITK MessageHandling extension (nested extensions). Keep generic for POC:
	Extension *MessageHandlingExtension `xml:"extension,omitempty"`

	Event *CodingEvent `xml:"event,omitempty"`

	Sender    *Reference `xml:"sender,omitempty"`
	Timestamp *Attr      `xml:"timestamp,omitempty"`

	Source *MessageSource `xml:"source,omitempty"`
	Focus  *Reference     `xml:"focus,omitempty"`
}

type MessageSource struct {
	Endpoint *Attr `xml:"endpoint,omitempty"`
}

type MessageHandlingExtension struct {
	XMLName    xml.Name         `xml:"extension"`
	Extensions []MHSubExtension `xml:"extension"`
}

type MHSubExtension struct {
	XMLName      xml.Name        `xml:"extension"`
	URL          string          `xml:"url,attr"`
	ValueBoolean *bool           `xml:"valueBoolean,omitempty"`
	ValueCoding  *ValueCoding    `xml:"valueCoding,omitempty"`
	ValueRef     *ValueReference `xml:"valueReference,omitempty"`
	ValueString  *Text           `xml:"valueString,omitempty"`
}

type BooleanSubExt struct {
	XMLName xml.Name `xml:"extension"`
	URL     string   `xml:"url,attr"`
	Value   *Attr    `xml:"valueBoolean,omitempty"`
}

type CodingSubExt struct {
	XMLName     xml.Name     `xml:"extension"`
	URL         string       `xml:"url,attr"`
	ValueCoding *ValueCoding `xml:"valueCoding,omitempty"`
}

type ReferenceSubExt struct {
	XMLName        xml.Name   `xml:"extension"`
	URL            string     `xml:"url,attr"`
	ValueReference *Reference `xml:"valueReference,omitempty"`
}

type StringSubExt struct {
	XMLName xml.Name `xml:"extension"`
	URL     string   `xml:"url,attr"`
	Value   *Attr    `xml:"valueString,omitempty"`
}

type ValueCoding struct {
	XMLName xml.Name `xml:"valueCoding"`
	System  *Attr    `xml:"system,omitempty"`
	Code    *Attr    `xml:"code,omitempty"`
	Display *Attr    `xml:"display,omitempty"`
}

type ValueReference struct {
	XMLName   xml.Name  `xml:"valueReference"`
	Reference Reference `xml:"reference"`
}

type Text struct {
	Value string `xml:"value,attr"`
}

type ValueString struct {
	XMLName xml.Name `xml:"valueString"`
	Text
}
