package main

import "encoding/xml"

// Adag is the root element of the dax file.
type Adag struct {
	XMLName  xml.Name `xml:"adag"`
	Jobs     []Job    `xml:"job"`
	Childs   []Child  `xml:"child"`
	ChildMap map[string][]string
}

// Job is a step in the DAG.
type Job struct {
	XMLName xml.Name `xml:"job"`
	ID      string   `xml:"id,attr"`
}

// Child is used to describe dependencies. A child has parents, which are its
// dependencies. Child.Ref maps to Job.ID.
type Child struct {
	XMLName xml.Name `xml:"child"`
	Ref     string   `xml:"ref,attr"`
	Parents []Parent `xml:"parent"`
}

// Parent is a dependency of a child.
type Parent struct {
	XMLName xml.Name `xml:"parent"`
	Ref     string   `xml:"ref,attr"`
}
