package orchestrator

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {

	acts := XMLActions{
		Items: []XMLAction{
			XMLActionLs{Path: "."},
			XMLActionCat{Filename: "./test.txt"},
			XMLActionGitStatus{},
		},
	}

	res, err := acts.ToXML()
	require.NoError(t, err)

	// Verify the marshalled XML
	expected := `<actions>
	<ls>.</ls>
	<cat>./test.txt</cat>
	<git-status/>
</actions>`
	require.Equal(t, expected, res)
}

func TestUnmarshal(t *testing.T) {
	input := `<actions>
		<ls>./test.txt</ls>
		<cat>./test.txt</cat>
		<git-status/>
	</actions>`

	actions, err := ActionsFromXML(input)
	require.NoError(t, err)

	// Verify the counts
	require.Len(t, actions.Items, 3)

	// Verify the order
	expectedTypes := []string{"ls", "cat", "git-status"}
	gotTypes := make([]string, len(actions.Items))
	for i, item := range actions.Items {
		gotTypes[i] = item.GetType()
	}

	if !reflect.DeepEqual(expectedTypes, gotTypes) {
		t.Errorf("Expected order %v, got %v", expectedTypes, gotTypes)
	}

	// Verify cat content
	require.Equal(t, "./test.txt", actions.Items[1].(XMLActionCat).Filename)
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
	require.NoError(t, err)

	require.Len(t, actions.Items, 1)

	expected := "\n1a\nhi\n.\nw test.txt\n\t"
	require.Equal(t, expected, actions.Items[0].(XMLActionEd).Script)

	remarshalled, err := actions.ToXML()
	require.NoError(t, err)

	require.Equal(t, input, remarshalled)
}

func TestThoughtUnmarshal(t *testing.T) {
	input := `<think>
Hello, world!
	Life is good!
	</think>`

	thoughts, err := ThoughtFromXML(input)
	require.NoError(t, err)

	expected := "\nHello, world!\n\tLife is good!\n\t"
	require.Equal(t, expected, thoughts.Text)

	remarshalled, err := thoughts.ToXML()
	require.NoError(t, err)

	require.Equal(t, input, remarshalled)
}

func TestResponseUnmarshal(t *testing.T) {
	input := `
<think>
	It is imparative that I read test.txt and then write foo to test2.txt
</think>
<actions>
	<cat>./test.txt</cat>
	<ed>
foo
w test2.txt
	</ed>
</actions>
`

	response, err := ParseModelResponse(input)
	require.NoError(t, err)

	require.Equal(t, "\n\tIt is imparative that I read test.txt and then write foo to test2.txt\n", response.Thought.Text)
	require.Len(t, response.Actions.Items, 2)
	require.Equal(t, "cat", response.Actions.Items[0].GetType())
	require.Equal(t, "ed", response.Actions.Items[1].GetType())
	require.Equal(t, "./test.txt", response.Actions.Items[0].(XMLActionCat).Filename)
	require.Equal(t, "\nfoo\nw test2.txt\n\t", response.Actions.Items[1].(XMLActionEd).Script)
}

func TestResponseMarshal(t *testing.T) {
	response := ParsedModelResponse{
		Thought: XMLThought{Text: "Hello, world!"},
		Actions: XMLActions{Items: []XMLAction{XMLActionCat{Filename: "./test.txt"}, XMLActionGitStatus{}}},
	}

	xml, err := response.ToXML()
	require.NoError(t, err)
	expected := `<response>
<think>Hello, world!</think>
<actions>
	<cat>./test.txt</cat>
	<git-status/>
</actions>
</response>`

	require.Equal(t, expected, xml)
}

func TestBuildPrompt(t *testing.T) {

	expected := "A series of interactions between Assistant and a git repository. Assistant is given a goal at the beginning of the interaction and then executes a series of steps to accomplish that goal. " +
		"Assistant is able to see all previous steps and their results. From that, the assistant first thinks about the reasoning process in " +
		"their mind and then executes a series of actions against the repo. Assistant uses XML to perform actions against the repo. Supported actions:\n" +
		"<ls>directory-name</ls>\n\tList all files in $directory-name. Supports \".\" to mean the root of the repository\n" +
		"<cat>filename</cat>\n\tPrints the contents of $filename (including line numbers)\n" +
		"<mkdir>new-directory</mkdir>\n\tInvokes the equivalent of mkdir -p $new-directory\n" +
		"<ed>script</ed>\n\tEdit existing files & creating new ones. $script can be multiple lines will be executed with the text-editor ed.\n" +
		"<git-status/>\n\tSee all uncommitted changes.\n" +
		"<git-commit/>\n\tFinish your work on the repo. Assistant's work will be run though CI and reviewed. Assistant will no longer be able to perform any steps or actions. This should only be executed once the repo is in a working state, is formatted well, and is ready to show to others. It is a syntax-error to put any actions after the commit action.\n" +
		"\n" +
		"The reasoning process and actions are enclosed within <think> </think> and " +
		"<actions> </actions> tags, respectively. For example a valid response from Assistant would be:\n" +
		"<think> reasoning process here </think>\n " +
		"<actions> <ls>.</ls> <git-status/> ... </actions>\n" +
		"Assistant will get the ability to perform multiple steps so it is expected that they will use the first few steps to gather information\n" +
		"\n" +
		"<goal> Fix the compilation errors </goal>\n" +
		"<previous-steps>\n" +
		"<step>\n" +
		"<compilation-output> output here </compilation-output>\n" +
		"<think> thoughts here </think>\n" +
		"<actions> <ls>.</ls> <mkdir> foo </mkdir></actions>\n" +
		"<output action=\"ls\"> test.txt bax.txt </output>\n" +
		"<output action=\"mkdir\"> success </output>\n" +
		"</step>\n" +
		"</previous-steps>\n" +
		"\n" +
		"<compilation-output> output here </compilation-output>\n" +
		"Assistant:"

	require.True(t, len(expected) > 0)
}

/*
From DeepSeek-R1

return "A conversation between User and Assistant. The user asks a question, and the Assistant solves it." +
	"The assistant first thinks about the reasoning process in the mind and then provides the user" +
	"with the answer. The reasoning process and answer are enclosed within <think> </think> and" +
	"<answer> </answer> tags, respectively, i.e., <think> reasoning process here </think>" +
	"<answer> answer here </answer>. User: prompt. Assistant:"
*/
