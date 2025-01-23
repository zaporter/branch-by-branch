package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/go-xmlfmt/xmlfmt"
)

// 🚩 This file uses XML Parsing from golang mostly for unmarshalling & prefers to do marshalling manually.
// That is mostly because we want fine-grained control over the prompt.

type XMLThought struct {
	XMLName xml.Name `xml:"think"`
	Text    string   `xml:",innerxml"`
}

// Action is an interface that all actions must implement
type XMLAction interface {
	GetType() string
}

type XMLActions struct {
	// XMLName is set in UnmarshalXML
	Items []XMLAction `xml:"-"`
}

// --- actions ---

type XMLActionHelp struct {
	XMLName xml.Name `xml:"help"`
}

func (a XMLActionHelp) GetType() string {
	return "help"
}

type XMLActionCat struct {
	XMLName  xml.Name `xml:"cat"`
	Filename string   `xml:",chardata"`
}

func (a XMLActionCat) GetType() string {
	return "cat"
}

type XMLActionEd struct {
	XMLName xml.Name `xml:"ed"`
	Script  string   `xml:",innerxml"`
}

func (a XMLActionEd) GetType() string {
	return "ed"
}

// UnmarshalXML implements custom unmarshalling to preserve order
func (a *XMLActions) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	a.Items = []XMLAction{}

	for {
		token, err := d.Token()
		if err != nil {
			return err
		}

		if end, ok := token.(xml.EndElement); ok && end.Name == start.Name {
			break
		}

		if start, ok := token.(xml.StartElement); ok {
			switch start.Name.Local {
			case "help":
				var help XMLActionHelp
				if err := d.DecodeElement(&help, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, help)
			case "cat":
				var cat XMLActionCat
				if err := d.DecodeElement(&cat, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, cat)
			case "ed":
				var ed XMLActionEd
				if err := d.DecodeElement(&ed, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, ed)
			}
		}
	}
	return nil
}

// MarshalXML implements custom marshalling to write the ordered items
func (a XMLActions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "actions"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// Encode each item in order
	for _, item := range a.Items {
		if err := e.Encode(item); err != nil {
			return err
		}
	}

	return e.EncodeToken(start.End())
}

func (a XMLActions) ToXML() (string, error) {
	xml, err := xml.Marshal(a)
	if err != nil {
		return "", err
	}
	// self-close some tags
	// https://github.com/golang/go/issues/21399
	asString := strings.ReplaceAll(string(xml), "<help></help>", "<help/>")
	// hackery to ensure Marshal(Unmarshal(input)) == input (mostly)
	// (got around this by using ,innerxml, however, this may not always work)
	/*
		asString = strings.ReplaceAll(asString, "&#xA;", "\n")
		asString = strings.ReplaceAll(asString, "<ed>", "<ed>\n")
		asString = strings.ReplaceAll(asString, "</ed>", "\n\t</ed>")
	*/
	formatted := xmlfmt.FormatXML(asString, "", "\t")
	// xmlfmt is inserting a leading \n for some reason
	formatted = strings.TrimSpace(formatted)
	return formatted, nil
}
func ActionsFromXML(input string) (XMLActions, error) {
	actions := XMLActions{}
	err := xml.Unmarshal([]byte(input), &actions)
	if err != nil {
		return XMLActions{}, err
	}
	return actions, nil
}

func ThoughtFromXML(input string) (XMLThought, error) {
	thought := XMLThought{}
	err := xml.Unmarshal([]byte(input), &thought)
	if err != nil {
		return XMLThought{}, err
	}
	return thought, nil
}

func (t XMLThought) ToXML() (string, error) {
	xml, err := xml.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(xml), nil
}

type ParsedModelResponse struct {
	Thought XMLThought
	Actions XMLActions
}

func ParseModelResponse(input string) (ParsedModelResponse, error) {
	lastCharacterAtEndOfThought := strings.Index(input, "</think>")
	if lastCharacterAtEndOfThought == -1 {
		return ParsedModelResponse{}, errors.New("no <think></think> second found in response")
	}
	thoughtInput := input[:lastCharacterAtEndOfThought+len("</think>")]
	actionsInput := input[lastCharacterAtEndOfThought+len("</think>"):]

	if len(strings.TrimSpace(actionsInput)) == 0 {
		return ParsedModelResponse{}, errors.New("no <actions></actions> found in response")
	}

	thought, err := ThoughtFromXML(thoughtInput)
	if err != nil {
		return ParsedModelResponse{}, err
	}
	actions, err := ActionsFromXML(actionsInput)
	if err != nil {
		return ParsedModelResponse{}, err
	}
	return ParsedModelResponse{Thought: thought, Actions: actions}, nil
}

func (r ParsedModelResponse) ToXML() (string, error) {
	thoughtXML, err := r.Thought.ToXML()
	if err != nil {
		return "", err
	}
	actionsXML, err := r.Actions.ToXML()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<response>\n%s\n%s\n</response>", thoughtXML, actionsXML), nil
}

type PreviousStep struct {
	CompilationOutput string
	ModelResponse     ParsedModelResponse
	Outputs           []ActionOutput
}

type PreviousSteps struct {
	Steps []PreviousStep
}

type State struct {
	Goal          string
	PreviousSteps PreviousSteps
}
