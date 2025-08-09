package orchestrator

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"
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
	// can be empty
	GetCompilationTask() string
	Validate() error
}

type XMLActions struct {
	// XMLName is set in UnmarshalXML
	Items []XMLAction `xml:"-"`
}

// --- actions ---

type XMLActionLs struct {
	XMLName xml.Name `xml:"ls"`
	Path    string   `xml:",chardata"`
}

func (a XMLActionLs) GetType() string {
	return "ls"
}

func (a XMLActionLs) GetCompilationTask() string {
	return fmt.Sprintf("ls --classify %q", a.Path)
}

func (a XMLActionLs) Validate() error {
	if a.Path == "" {
		return errors.New("path is required")
	}
	if !filepath.IsLocal(a.Path) {
		return errors.New("path must be relative to the current directory")
	}
	return nil
}

type XMLActionGrep struct {
	XMLName xml.Name `xml:"grep"`
	Pattern string   `xml:",chardata"`
}

func (a XMLActionGrep) GetType() string {
	return "grep"
}

func (a XMLActionGrep) GetCompilationTask() string {
	return fmt.Sprintf("grep --exclude-dir=.git --exclude-dir=.lake -n --perl -R %q", a.Pattern)
}

func (a XMLActionGrep) Validate() error {
	if a.Pattern == "" {
		return errors.New("pattern is required")
	}
	return nil
}

type XMLActionCat struct {
	XMLName  xml.Name `xml:"cat"`
	Filename string   `xml:",chardata"`
}

func (a XMLActionCat) GetType() string {
	return "cat"
}

func (a XMLActionCat) GetCompilationTask() string {
	return fmt.Sprintf("cat --number %q", a.Filename)
}

func (a XMLActionCat) Validate() error {
	if a.Filename == "" {
		return errors.New("filename is required")
	}
	if !filepath.IsLocal(a.Filename) {
		return errors.New("filename must be relative to the current directory")
	}
	return nil
}

type XMLActionMkdir struct {
	XMLName xml.Name `xml:"mkdir"`
	Path    string   `xml:",chardata"`
}

func (a XMLActionMkdir) GetType() string {
	return "mkdir"
}

func (a XMLActionMkdir) GetCompilationTask() string {
	return fmt.Sprintf("mkdir -p %q", a.Path)
}

func (a XMLActionMkdir) Validate() error {
	if a.Path == "" {
		return errors.New("path is required")
	}
	if !filepath.IsLocal(a.Path) {
		return errors.New("path must be relative to the current directory")
	}
	return nil
}

type XMLActionEd struct {
	XMLName xml.Name `xml:"ed"`
	Script  string   `xml:",innerxml"`
}

func (a XMLActionEd) GetType() string {
	return "ed"
}

func (a XMLActionEd) GetCompilationTask() string {
	return fmt.Sprintf("cat << 'EOF' | ed\n%s\nEOF", strings.TrimSpace(a.Script))
}

func (a XMLActionEd) Validate() error {
	if a.Script == "" {
		return errors.New("script is required")
	}
	if strings.Contains(a.Script, "EOF") {
		return errors.New("script cannot contain EOF")
	}
	lines := strings.Split(a.Script, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			return errors.New("script cannot have command lines (lines starting with !)")
		}
	}
	return nil
}

type XMLActionGitStatus struct {
	XMLName xml.Name `xml:"git-status"`
}

func (a XMLActionGitStatus) GetType() string {
	return "git-status"
}

func (a XMLActionGitStatus) GetCompilationTask() string {
	// special. replaced during execution.
	return ""
}

func (a XMLActionGitStatus) Validate() error {
	return nil
}

type XMLActionGitCommit struct {
	XMLName xml.Name `xml:"git-commit"`
}

func (a XMLActionGitCommit) GetType() string {
	return "git-commit"
}

func (a XMLActionGitCommit) GetCompilationTask() string {
	// special. replaced during execution.
	return ""
}

func (a XMLActionGitCommit) Validate() error {
	return nil
}

type XMLActionAbort struct {
	XMLName xml.Name `xml:"abort"`
}

func (a XMLActionAbort) GetType() string {
	return "abort"
}

func (a XMLActionAbort) GetCompilationTask() string {
	return "echo ABORTED"
}
func (a XMLActionAbort) Validate() error {
	return nil
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
			case "ls":
				var item XMLActionLs
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "cat":
				var item XMLActionCat
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "mkdir":
				var item XMLActionMkdir
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "ed":
				var item XMLActionEd
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "grep":
				var item XMLActionGrep
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "git-status":
				var item XMLActionGitStatus
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "git-commit":
				var item XMLActionGitCommit
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
			case "abort":
				var item XMLActionAbort
				if err := d.DecodeElement(&item, &start); err != nil {
					return err
				}
				a.Items = append(a.Items, item)
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
	asString := string(xml)
	// self-close some tags
	// https://github.com/golang/go/issues/21399
	asString = strings.ReplaceAll(asString, "<git-status></git-status>", "<git-status/>")
	asString = strings.ReplaceAll(asString, "<git-commit></git-commit>", "<git-commit/>")
	asString = strings.ReplaceAll(asString, "<abort></abort>", "<abort/>")
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

// Validate ensures that if git-commit is present, then it is the last action
// and that abort is the only action if present.
func (actions *XMLActions) Validate() error {
	for i, action := range actions.Items {
		if action.GetType() == "abort" {
			if len(actions.Items) != 1 {
				return errors.New("abort must be the only action if present")
			}
		}
		if action.GetType() == "git-commit" {
			if i != len(actions.Items)-1 {
				return errors.New("git-commit must be the last action")
			}
		}
		err := action.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

func (actions *XMLActions) ContainsGitCommit() bool {
	if len(actions.Items) == 0 {
		return false
	}
	return actions.Items[len(actions.Items)-1].GetType() == "git-commit"
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
