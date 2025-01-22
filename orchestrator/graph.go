package main

type Goal string

// IDEA:
// I suggest that instead of having one large graph (like sudoku),
// that I should lean into smaller graphs (the odds of state-equivalence are much lower)
// This means that the graph can more clearly branch off from some commit.
// Then we generate a tree from there

type (
	TaskList struct {
		Tasks []Task
	}
	Task struct {
		ID       string
		AddProof *struct {
			ProofText string
		}
		Format *struct{}
	}
)

type (
	Graph struct {
	}

	Node struct {
		ID string
		// Branches are read-only after creation
		// So read-only nodes can share a branch
		GitBranch string
	}
	PerformedAction struct {
		ID string
	}
)

type (
	TrainableDatapoint struct {
		Prompt string
		Result string
		Reward float64
	}
)

type PromptOpts struct {
	ActionResult string
}

/*
return "A conversation between User and Assistant. The user asks a question, and the Assistant solves it." +
	"The assistant first thinks about the reasoning process in the mind and then provides the user" +
	"with the answer. The reasoning process and answer are enclosed within <think> </think> and" +
	"<answer> </answer> tags, respectively, i.e., <think> reasoning process here </think>" +
	"<answer> answer here </answer>. User: prompt. Assistant:"
*/

func GeneratePrompt(opts PromptOpts) string {
	return "A series of interactions between Assistant and a git repo. The repo starts in a bad state and " +
		"the Assistant fixes it via a series of steps. The Assistant first thinks about the reasoning process in their mind " +
		"and then executes a series of actions against the repo. The reasoning process and actions are enclosed within <think> </think> and " +
		"<action> </action> tags, respectively, i.e. " +
		"<think> reasoning process here </think> <action> action here </action> <action> second action here </action> ... " +
		"The Assistant will get the ability to perform multiple steps so it is expected that they will use the first few steps to gather information " +
		"and then only solve & push the solution once they are confident in the solution and think it fits in well with the rest of the codebase.\n" +
		// Previous iterations
		"<previous-steps> " +
		"<step> \n" +
		// step
		"<issue>" + "{lean compilation error}" + "</issue>" +
		"<output>" + "{output of reading from file}" + "</output>" +
		"<assistant-output>" + "{output from last step}" + "</assistant-output>" +
		"</step>" +
		"</previous-steps> " +
		// end
		"<issue> " + "{lean compliation error}" + " </issue> " +
		"Assistant:"
}
