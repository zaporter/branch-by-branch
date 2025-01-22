package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)

	// Verify the marshalled XML
	expected := `<actions>
	<help/>
	<cat>./test.txt</cat>
	<help/>
</actions>`
	assert.Equal(t, expected, res)
}

func TestUnmarshal(t *testing.T) {
	input := `<actions>
		<help/>
		<cat>./test.txt</cat>
		<help/>
	</actions>`

	actions, err := ActionsFromXML(input)
	assert.NoError(t, err)

	// Verify the counts
	assert.Len(t, actions.Items, 3)

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
	assert.Equal(t, "./test.txt", actions.Items[1].(XMLActionCat).Filename)
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

	actions, err := ActionsFromXML(input)
	assert.NoError(t, err)

	assert.Len(t, actions.Items, 1)

	expected := "\n1a\nhi\n.\nw test.txt\n\t"
	assert.Equal(t, expected, actions.Items[0].(XMLActionEd).Script)

	remarshalled, err := actions.ToXML()
	assert.NoError(t, err)

	assert.Equal(t, input, remarshalled)
}

func TestThoughtUnmarshal(t *testing.T) {
	input := `<think>
Hello, world!
	Life is good!
	</think>`

	thoughts, err := ThoughtFromXML(input)
	assert.NoError(t, err)

	expected := "\nHello, world!\n\tLife is good!\n\t"
	assert.Equal(t, expected, thoughts.Text)

	remarshalled, err := thoughts.ToXML()
	assert.NoError(t, err)

	assert.Equal(t, input, remarshalled)
}
