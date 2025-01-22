package main

import (
	"reflect"
	"testing"
)

func TestMarshal(t *testing.T) {

	acts := XMLActions{
		Items: []XMLAction{
			XMLActionHelp{},
			XMLActionCat{Filename: "./test.txt"},
			XMLActionHelp{},
		},
	}

	res, err := acts.ToXML()
	if err != nil {
		t.Fatal(err)
	}

	// Verify the marshalled XML
	expected := `<actions>
	<help/>
	<cat>./test.txt</cat>
	<help/>
</actions>`
	if res != expected {
		t.Errorf("Expected %s, got %s", expected, res)
	}
}

func TestUnmarshal(t *testing.T) {
	input := `<actions>
		<help/>
		<cat>./test.txt</cat>
		<help/>
	</actions>`

	actions, err := FromXML(input)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the counts
	if len(actions.Items) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions.Items))
	}

	// Verify the order
	expectedTypes := []string{"help", "cat", "help"}
	gotTypes := make([]string, len(actions.Items))
	for i, item := range actions.Items {
		gotTypes[i] = item.GetType()
	}

	if !reflect.DeepEqual(expectedTypes, gotTypes) {
		t.Errorf("Expected order %v, got %v", expectedTypes, gotTypes)
	}

	// Verify cat content
	if actions.Items[1].(XMLActionCat).Filename != "./test.txt" {
		t.Errorf("Expected './test.txt', got '%s'", actions.Items[1].(XMLActionCat).Filename)
	}
}

func TestEdUnmarshal(t *testing.T) {
	input := `<actions>
	<ed>
1a
hi
.
w test.txt
	</ed>
</actions>`

	actions, err := FromXML(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(actions.Items) != 1 {
		t.Errorf("Expected 1 action, got %d", len(actions.Items))
	}

	if actions.Items[0].(XMLActionEd).Script != "1a\nhi\n.\nw test.txt" {
		t.Errorf("Expected '1a\nhi\n.\nw test.txt', got '%s'", actions.Items[0].(XMLActionEd).Script)
	}
	remarshalled, err := actions.ToXML()
	if err != nil {
		t.Fatal(err)
	}
	if remarshalled != input {
		t.Errorf("Expected %s, got %s", input, remarshalled)
	}
}
