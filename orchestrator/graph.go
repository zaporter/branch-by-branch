package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mroth/weightedrand/v2"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

type NodeState string

const (
	NodeStateAwaitingGoalSetup NodeState = "node_awaiting_goal_setup"
	NodeStateRunningGoalSetup  NodeState = "node_state_running_goal_setup"
	// Will be queued to be compiled
	NodeStateAwaitingCompilation NodeState = "node_awaiting_compilation"
	NodeStateRunningCompilation  NodeState = "node_state_running_compilation"
	// Will be queued to have inference run (producing children)
	NodeStateAwaitingInference NodeState = "node_awaiting_inference"
	NodeStateRunningInference  NodeState = "node_state_running_inference"
	// All nodes will rest here until the graph is finished
	// At that point, a reward can be calculated for each child
	// NodeResult should be calculated at this point (it may remain none for non-terminal nodes)
	NodeStateDone NodeState = "node_state_done"
)

// collapsed version of NodeState
type NodeStage string

const (
	NodeStageGoalSetup   NodeStage = "node_stage_goal_setup"
	NodeStageCompilation NodeStage = "node_stage_compilation"
	NodeStageInference   NodeStage = "node_stage_inference"
	NodeStageDone        NodeStage = "node_stage_done"
)

func NodeStageFromState(state NodeState) NodeStage {
	switch state {
	case NodeStateAwaitingGoalSetup:
		fallthrough
	case NodeStateRunningGoalSetup:
		return NodeStageGoalSetup
	case NodeStateAwaitingCompilation:
		fallthrough
	case NodeStateRunningCompilation:
		return NodeStageCompilation
	case NodeStateAwaitingInference:
		fallthrough
	case NodeStateRunningInference:
		return NodeStageInference
	case NodeStateDone:
		return NodeStageDone
	}
	panic(fmt.Sprintf("unknown state %s", state))
}

// The graph tries to be as stateless as possible
// However, there are serious ergonomic & performance benefits
// To these extra states. We just have to reset them in order to resume from a saved graph
var TransientNodeStateResetMap = map[NodeState]NodeState{
	NodeStateRunningGoalSetup:   NodeStateAwaitingGoalSetup,
	NodeStateRunningCompilation: NodeStateAwaitingCompilation,
	NodeStateRunningInference:   NodeStateAwaitingInference,
}

type NodeResult string

const (
	// non-terminal node or in-progress
	NodeResultNone NodeResult = "node_result_none"
	// 🤙
	NodeResultSuccess NodeResult = "node_result_success"
	// The model said it was done when it wasn't
	NodeResultFailure NodeResult = "node_result_failure"
	// bad action syntax -> instant failure
	NodeResultSyntaxFailure NodeResult = "node_result_syntax_failure"
	// max depth exceeded
	NodeResultDepthExhaustionFailure NodeResult = "node_result_depth_exhaustion"
	// max context length exceeded
	NodeResultContextExhaustionFailure NodeResult = "node_result_context_exhaustion"
	// was in a non-terminal state but had been requested to be terminated
	NodeResultTerminated NodeResult = "node_result_terminated"
)

type GraphState string

const (
	GraphStateAwaitingGoalSetup GraphState = "graph_awaiting_goal_setup"
	GraphStateInProgress        GraphState = "graph_in_progress"
	GraphStateSuccess           GraphState = "graph_success"
	GraphStateFailed            GraphState = "graph_failed"
	// May not count against the weighting of the branch target
	// (depending on gracious I am feeling)
	GraphStateGoalSetupFailed GraphState = "graph_goal_setup_failed"
)

type RepoGraph struct {
	ID                  RepoGraphID                           `json:"id"`
	BranchTargets       map[BranchName]*RepoGraphBranchTarget `json:"branch_targets"`
	ShouldAdvertiseChan chan CommitGraphLocator               `json:"-"`
	// TODO: This should be passed through via func params.
	Ctx context.Context `json:"-"`
}

type RepoGraphBranchTarget struct {
	BranchName BranchName `json:"branch_name"`
	CreatedAt  time.Time  `json:"created_at"`
	// nil if this is the root
	ParentBranchName *BranchName `json:"parent_branch_name,omitempty"`
	// Goal that was used to create this branch target (nil if this is the root)
	TraversalGoalID *GoalID                 `json:"traversal_goal_id,omitempty"`
	Subgraphs       map[GoalID]*CommitGraph `json:"subgraphs"`
}

// See comment in BuildCompilationTasksForNode about git-commit
// to understand this more
type CGResult struct {
	// This is also CG[GeneratingNodes[0]].BranchName
	BranchTarget BranchName `json:"branch_target"`
	// can be used as a unique identifier within the CG results array
	DiffPatch       string   `json:"diff_patch"`
	GeneratingNodes []NodeID `json:"generating_nodes"`
}

// model used to create the node.
// used mostly for bookkeeping & manual validation
type ModelReference struct {
	// doesn't include /base suffix.
	ModelName string `json:"model_name"`
	Adapter   string `json:"model_adapter"`
}

type CommitGraph struct {
	// goal_id is a unique identifier for the graph because
	// we will never start a new graph at the same target on the same goal
	GoalID   GoalID                      `json:"goal_id"`
	RootNode NodeID                      `json:"root_node"`
	Nodes    map[NodeID]*CommitGraphNode `json:"nodes"`
	State    GraphState                  `json:"state"`

	Results []*CGResult `json:"results"`
}

type CommitGraphNode struct {
	ID        NodeID    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Depth     int       `json:"depth"`
	// nil if this is the root
	Parent               *NodeID        `json:"parent"`
	Children             []NodeID       `json:"children"`
	State                NodeState      `json:"state"`
	Result               NodeResult     `json:"result"`
	TerminationRequested bool           `json:"termination_requested"`
	Metadata             NodeMetadata   `json:"metadata"`
	ModelReference       ModelReference `json:"model_reference"`

	// The inference result that LED to the creation of this node.
	// Empty if this is the root
	InferenceOutput string `json:"inference_output"`
	// The results of the computation from apply(parse(inference_output) @ parent.branch_name)
	ActionOutputs []ActionOutput `json:"action_outputs"`
	// The results of the compilation
	CompilationResult *CompilationResult `json:"compilation_result"`
	// apply(parse(inference_output) @ parent.branch_name) is written to branch_name
	// (unless this is the root, in which case, it is:
	// apply(goals[goal_id].GoalStatement @ branch_target.branch_name))
	BranchName BranchName `json:"branch_name"`
}

// Manual data that I have added from the webui.
// normal execution should probably not populate these fields.
// add fields to CommitGraphNode instead.
// Make sure this is kept up to date with the webui (otherwise the webui will accidentally overwrite the data)
type NodeMetadata struct {
	WasManuallyCreated bool   `json:"was_manually_created,omitempty"`
	IsFavorite         bool   `json:"is_favorite,omitempty"`
	IsGoldenSample     bool   `json:"is_golden_sample,omitempty"`
	Label              string `json:"label,omitempty"`
}

// not an action, but a way to return output from an action
// Probably could be merged with CompilationResult
type ActionOutput struct {
	ActionName string `json:"action_name"`
	Text       string `json:"text"`
	ExitCode   int    `json:"exit_code"`
}

func NewCommitGraph(goalID GoalID) *CommitGraph {
	rootNode := &CommitGraphNode{
		ID:         NewNodeID(),
		CreatedAt:  time.Now(),
		Depth:      0,
		Parent:     nil,
		Children:   []NodeID{},
		State:      NodeStateAwaitingGoalSetup,
		Result:     NodeResultNone,
		BranchName: NewBranchName(),
		Metadata:   NodeMetadata{},
	}
	return &CommitGraph{
		GoalID:   goalID,
		RootNode: rootNode.ID,
		Nodes:    map[NodeID]*CommitGraphNode{rootNode.ID: rootNode},
		State:    GraphStateAwaitingGoalSetup,
		Results:  []*CGResult{},
	}
}

func NewRepoGraph(rootBranchName BranchName) *RepoGraph {
	rg := &RepoGraph{
		ID: NewRepoGraphID(),
		BranchTargets: map[BranchName]*RepoGraphBranchTarget{
			rootBranchName: {
				CreatedAt:        time.Now(),
				BranchName:       rootBranchName,
				ParentBranchName: nil,
				TraversalGoalID:  nil,
				Subgraphs:        map[GoalID]*CommitGraph{},
			},
		},
		ShouldAdvertiseChan: make(chan CommitGraphLocator, 64),
	}
	return rg
}

func (rg *RepoGraph) SaveToFile(path string) error {
	json, err := json.Marshal(rg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, json, 0644)
}

func (rg *RepoGraph) LoadFromFile(path string) error {
	str, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(str, rg)
	if err != nil {
		return err
	}
	return nil
}
func (rg *RepoGraph) ResetTransientStates() {
	for _, branchTarget := range rg.BranchTargets {
		for _, subgraph := range branchTarget.Subgraphs {
			for _, node := range subgraph.Nodes {
				if _, ok := TransientNodeStateResetMap[node.State]; ok {
					node.State = TransientNodeStateResetMap[node.State]
				}
			}
		}
	}
}

// implies success
// may not actually create a new branch if the diffpatch is duplicative
func (rg *RepoGraph) CreateBranchTargetFromNode(slice NodeSlice, gitCommitDiffPatch string) {
	sourceBranchName := slice.BranchTarget.BranchName
	newBranchName := slice.CommitGraphNode.BranchName
	traversalGoalID := slice.CommitGraph.GoalID

	for _, result := range slice.CommitGraph.Results {
		if gitCommitDiffPatch == result.DiffPatch {
			// this is a duplicate
			result.GeneratingNodes = append(result.GeneratingNodes, slice.CommitGraphNode.ID)
			return
		}
	}

	slice.CommitGraph.Results = append(slice.CommitGraph.Results, &CGResult{
		BranchTarget:    newBranchName,
		DiffPatch:       gitCommitDiffPatch,
		GeneratingNodes: []NodeID{slice.CommitGraphNode.ID},
	})

	rg.BranchTargets[newBranchName] = &RepoGraphBranchTarget{
		CreatedAt:        time.Now(),
		BranchName:       newBranchName,
		ParentBranchName: &sourceBranchName,
		TraversalGoalID:  &traversalGoalID,
		Subgraphs:        map[GoalID]*CommitGraph{},
	}
}

func (rg *RepoGraph) HandleSetupCompilationOutput(logger *zerolog.Logger, locator NodeLocator, result *CompilationTaskResponse, goalProvider GoalProvider) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	if slice.CommitGraphNode.State != NodeStateRunningGoalSetup {
		return fmt.Errorf("node %v is not in state RunningGoalSetup", locator)
	}
	if slice.CommitGraph.State != GraphStateAwaitingGoalSetup {
		return fmt.Errorf("graph %v is not in state AwaitingGoalSetup", locator)
	}

	goal := goalProvider.GetGoal(slice.CommitGraph.GoalID)
	ok := goal.ValidateSetup(*result)
	if !ok {
		logger.Error().Msgf("goal setup failed for %s on branch %s to branch %s", slice.CommitGraph.GoalID, slice.BranchTarget.BranchName, slice.CommitGraphNode.BranchName)
		slice.CommitGraph.State = GraphStateGoalSetupFailed
		return nil
	}

	// goal compilation result is expected to be fed into the root node
	slice.CommitGraphNode.CompilationResult = &result.CompilationResult
	slice.CommitGraphNode.State = NodeStateAwaitingInference
	slice.CommitGraphNode.Result = NodeResultNone
	rg.tickUpdateCommitGraph(slice.AsCommitGraphSlice())
	return nil
}

func (rg *RepoGraph) HandleInferenceOutput(locator NodeLocator, result *InferenceTaskResponse) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	if slice.CommitGraphNode.State == NodeStateDone && slice.CommitGraphNode.Result == NodeResultTerminated {
		return nil
	}
	if slice.CommitGraphNode.State != NodeStateRunningInference {
		return fmt.Errorf("node %v is not in state RunningInference", locator)
	}
	if slice.CommitGraph.State != GraphStateInProgress {
		return fmt.Errorf("graph %v is not in state InProgress", locator)
	}
	node := slice.CommitGraphNode
	node.State = NodeStateDone
	// if we have children, we are (by definition) non-terminal
	node.Result = NodeResultNone
	for _, seq := range result.ReturnSequences {
		if _, err := rg.AddNodeToCommitGraph(locator, seq, NodeMetadata{}); err != nil {
			return err
		}
	}
	// add nodeToCommitGraph will tickUpdateCommitGraph
	return nil
}

func (rg *RepoGraph) AddNodeToCommitGraph(parentLocator NodeLocator, inferenceOutput string, metadata NodeMetadata) (NodeLocator, error) {
	parentSlice, err := rg.GetNodeSlice(parentLocator)
	if err != nil {
		return NodeLocator{}, err
	}
	parentNode := parentSlice.CommitGraphNode
	newNode := &CommitGraphNode{
		ID:              NewNodeID(),
		CreatedAt:       time.Now(),
		Depth:           parentNode.Depth + 1,
		Parent:          &parentNode.ID,
		Children:        []NodeID{},
		State:           NodeStateAwaitingCompilation,
		Result:          NodeResultNone,
		InferenceOutput: inferenceOutput,
		ActionOutputs:   []ActionOutput{},
		BranchName:      NewBranchName(),
		Metadata:        metadata,
	}
	// check for syntax errors
	// (easier here before pulling this off to queue)
	parsed, err := ParseModelResponse(inferenceOutput)
	if err != nil || parsed.Actions.Validate() != nil {
		newNode.State = NodeStateDone
		newNode.Result = NodeResultSyntaxFailure
	}
	parentSlice.CommitGraph.Nodes[newNode.ID] = newNode
	parentNode.Children = append(parentNode.Children, newNode.ID)
	rg.tickUpdateCommitGraph(parentSlice.AsCommitGraphSlice())
	return NodeLocatorFromTriplet(parentSlice.BranchTarget.BranchName, parentSlice.CommitGraph.GoalID, newNode.ID), nil
}

func (rg *RepoGraph) HandleCompilationOutput(locator NodeLocator, result *CompilationTaskResponse, maxDepth int) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	if slice.CommitGraphNode.State == NodeStateDone && slice.CommitGraphNode.Result == NodeResultTerminated {
		return nil
	}
	if slice.CommitGraphNode.State != NodeStateRunningCompilation {
		return fmt.Errorf("node %v is not in state RunningCompilation", locator)
	}
	if slice.CommitGraph.State != GraphStateInProgress {
		return fmt.Errorf("graph %v is not in state InProgress", locator)
	}
	node := slice.CommitGraphNode

	gitCommitDiffPatch := ""
	for _, res := range result.PreCommandsResults {
		node.ActionOutputs = append(node.ActionOutputs, ActionOutput{
			ActionName: res.ActionName,
			Text:       res.Out,
			ExitCode:   res.ExitCode,
		})
		if res.ActionName == "git-commit" && res.ExitCode == 0 {
			gitCommitDiffPatch = res.Out
		}
	}
	node.CompilationResult = &result.CompilationResult

	if gitCommitDiffPatch != "" {
		node.State = NodeStateDone
		if node.CompilationResult.ExitCode == 0 {
			node.Result = NodeResultSuccess
			// ❤️ create a new branch target!
			rg.CreateBranchTargetFromNode(slice, gitCommitDiffPatch)
		} else {
			node.Result = NodeResultFailure
		}
	} else if node.Depth >= maxDepth {
		node.State = NodeStateDone
		node.Result = NodeResultDepthExhaustionFailure
	} else {
		// After compiling the output, we are able to produce a new prompt
		node.State = NodeStateAwaitingInference
	}

	rg.tickUpdateCommitGraph(slice.AsCommitGraphSlice())

	return nil
}

// internal
func (rg *RepoGraph) tickUpdateCommitGraph(slice CommitGraphSlice) {
	// If all nodes are done, we can determine if the graph is successful
	if len(slice.CommitGraph.Nodes) == len(slice.CommitGraph.AllNodesInState(NodeStateDone)) {
		hasSuccess := false
		for _, node := range slice.CommitGraph.Nodes {
			if node.Result == NodeResultSuccess {
				hasSuccess = true
			}
		}
		if hasSuccess {
			slice.CommitGraph.State = GraphStateSuccess

			if rg.ShouldAdvertiseChan != nil {
				select {
				case rg.ShouldAdvertiseChan <- CommitGraphLocator{
					BranchTargetLocator: BranchTargetLocator{BranchName: slice.BranchTarget.BranchName},
					GoalID:              slice.CommitGraph.GoalID,
				}:
				case <-rg.Ctx.Done():
					return
				}
			}
		} else {
			slice.CommitGraph.State = GraphStateFailed
		}
	} else {
		rootNode, ok := slice.CommitGraph.Nodes[slice.CommitGraph.RootNode]
		if !ok {
			panic(fmt.Sprintf("root node %v not found", slice.CommitGraph.RootNode))
		}
		rootStage := NodeStageFromState(rootNode.State)
		if rootStage == NodeStageGoalSetup {
			slice.CommitGraph.State = GraphStateAwaitingGoalSetup
		} else {
			slice.CommitGraph.State = GraphStateInProgress
		}
	}

}

func (rg *RepoGraph) UnfinishedGraphs() []CommitGraphLocator {
	unfinishedGraphs := []CommitGraphLocator{}
	for branchName, branchTarget := range rg.BranchTargets {
		for goalID, subgraph := range branchTarget.Subgraphs {
			if subgraph.State == GraphStateInProgress || subgraph.State == GraphStateAwaitingGoalSetup {
				unfinishedGraphs = append(unfinishedGraphs,
					CommitGraphLocator{
						BranchTargetLocator: BranchTargetLocator{BranchName: branchName},
						GoalID:              goalID,
					})
			}
		}
	}
	return unfinishedGraphs
}

// Note: always call with depth 0
func (rg *RepoGraph) RequestNodeTerminationRecursively(nodeLocator NodeLocator, depth int) error {
	slice, err := rg.GetNodeSlice(nodeLocator)
	if err != nil {
		return err
	}
	node, ok := slice.CommitGraph.Nodes[nodeLocator.NodeID]
	if !ok {
		return fmt.Errorf("node %v not found", nodeLocator.NodeID)
	}
	node.TerminationRequested = true
	if node.State != NodeStateDone {
		node.State = NodeStateDone
		node.Result = NodeResultTerminated
	}
	for _, child := range node.Children {
		if err := rg.RequestNodeTerminationRecursively(NodeLocator{
			CommitGraphLocator: nodeLocator.CommitGraphLocator,
			NodeID:             child,
		}, depth+1); err != nil {
			return err
		}
	}
	// only tick the graph if we are at the root
	if depth == 0 {
		rg.tickUpdateCommitGraph(slice.AsCommitGraphSlice())
	}
	return nil
}

func (cg *CommitGraph) AllNodesInState(state NodeState) []*CommitGraphNode {
	nodes := []*CommitGraphNode{}
	for _, node := range cg.Nodes {
		if node.State == state {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (rg *RepoGraph) BuildInferenceTaskForNode(nodeLocator NodeLocator, goalProvider GoalProvider) (InferenceTask, error) {
	slice, err := rg.GetNodeSlice(nodeLocator)
	if err != nil {
		return InferenceTask{}, err
	}
	if slice.CommitGraphNode.Result == NodeResultSyntaxFailure {
		return InferenceTask{}, fmt.Errorf("cannot build inference task for node %v because it has syntax failure", slice.CommitGraphNode.ID)
	}

	var sb strings.Builder

	// Write the base prompt explaining the interaction model
	sb.WriteString("A series of interactions between Assistant and a git repository. Assistant is given a goal at the beginning of the interaction and then executes a series of steps to accomplish that goal. ")
	sb.WriteString("Assistant is able to see all previous steps and their results. From that, the assistant first thinks about the reasoning process in ")
	sb.WriteString("their mind and then executes a series of actions against the repo. Assistant uses XML to perform actions against the repo. Supported actions:\n")
	sb.WriteString("<ls>directory-name</ls>\n\tList all files in $directory-name. Supports \".\" to mean the root of the repository\n")
	sb.WriteString("<cat>filename</cat>\n\tPrints the contents of $filename (including line numbers)\n")
	sb.WriteString("<mkdir>new-directory</mkdir>\n\tInvokes the equivalent of mkdir -p $new-directory\n")
	sb.WriteString("<ed>script</ed>\n\tEdit existing files & creating new ones. $script can be multiple lines and will be executed with the text-editor ed (without a default open file).\n")
	sb.WriteString("<grep>pattern</grep>\n\tSearch for $pattern in all files in the repo. (Supports perl-compatible regex)\n")
	sb.WriteString("<git-status/>\n\tSee all uncommitted changes.\n")
	sb.WriteString("<git-commit/>\n\tFinish your work on the repo. Assistant's work will be run though CI and reviewed. Assistant will no longer be able to perform any steps or actions. This should only be executed once the repo is in a working state, is formatted well, and is ready to show to others. It is a syntax-error to put any actions after the commit action.\n")
	sb.WriteString("\n")
	sb.WriteString("The reasoning process and actions are enclosed within <think> </think> and ")
	sb.WriteString("<actions> </actions> tags, respectively. For example a valid response from Assistant would be:\n")
	sb.WriteString("<think> reasoning process here </think>\n ")
	sb.WriteString("<actions> <ls>.</ls> <git-status/> ... </actions>\n")
	sb.WriteString("\n")
	// sb.WriteString("IMPORTANT HINTS:\n")
	// sb.WriteString("\tTest.lean is READ ONLY. ASSISTANT CANNOT WRITE TO IT. The proof must be added to the Corelib directory.\n")
	// sb.WriteString("\tWhen you finish editing a file, you must end your script with 'w filetowrite.lean' otherwise ed won't know which file to write to.\n")
	// sb.WriteString("\tI recommend reading and editing Corelib/Data/Nat/Basic.lean and/or Corelib/Data/Nat/Defs.lean. That will help you solve the problem.\n")
	// sb.WriteString("\n")
	sb.WriteString("Assistant will get the ability to perform multiple steps so it is expected that they will use the first few steps to gather information\n\n")

	goal := goalProvider.GetGoal(slice.CommitGraph.GoalID)
	// Write the goal
	sb.WriteString(fmt.Sprintf("<goal> %s </goal>\n", goal.GoalStatement()))

	// we need to traverse up the graph to get the previous steps
	// and then reverse so we write in grandparent->parent->child order
	currentNode := slice.CommitGraphNode
	parents := []CommitGraphNode{*currentNode}
	for currentNode.Parent != nil {
		parents = append(parents, *slice.CommitGraph.Nodes[*currentNode.Parent])
		currentNode = slice.CommitGraph.Nodes[*currentNode.Parent]
	}
	leanOutputParams := StripParams{
		stripErrors:   false,
		stripInfos:    true,
		stripTraces:   true,
		stripWarnings: false,
	}
	stripEndNewline := func(s string) string {
		return strings.TrimRight(s, "\n")
	}
	if len(parents) > 1 {
		sb.WriteString("<previous-steps>\n")
		for i := len(parents) - 2; i >= 0; i-- {
			grandParent := parents[i+1]
			parentNode := parents[i]
			sb.WriteString(fmt.Sprintf("<step num=\"%v\">\n", len(parents)-i-1))

			if grandParent.CompilationResult != nil {
				sb.WriteString(fmt.Sprintf("<compilation-output code=\"%d\">\n%s\n</compilation-output>\n",
					grandParent.CompilationResult.ExitCode, stripEndNewline(stripLeanOutput(grandParent.CompilationResult.Out, leanOutputParams))))
			}

			// TODO: Should we parse and pretty-print this? > yes
			parsed, err := ParseModelResponse(parentNode.InferenceOutput)
			if err != nil {
				return InferenceTask{}, fmt.Errorf("node %v has non-parsable inference output %w", parentNode.ID, err)
			}
			thoughtXML, err := parsed.Thought.ToXML()
			if err != nil {
				return InferenceTask{}, fmt.Errorf("node %v has non-parsable thought %w", parentNode.ID, err)
			}
			sb.WriteString(thoughtXML)
			sb.WriteString("\n")
			actionsXML, err := parsed.Actions.ToXML()
			if err != nil {
				return InferenceTask{}, fmt.Errorf("node %v has non-parsable actions %w", parentNode.ID, err)
			}
			sb.WriteString(actionsXML)
			sb.WriteString("\n")

			// Add action outputs
			for _, output := range parentNode.ActionOutputs {
				if strings.HasSuffix(output.ActionName, "hidden") {
					continue
				}
				sb.WriteString(fmt.Sprintf("<output action=\"%s\" code=\"%d\">\n%s\n</output>\n",
					output.ActionName, output.ExitCode, stripEndNewline(output.Text)))
			}

			sb.WriteString("</step>\n")
		}
		sb.WriteString("</previous-steps>\n\n")
	}

	// Write the current compilation output if it exists
	if slice.CommitGraphNode.CompilationResult != nil {
		sb.WriteString(fmt.Sprintf("<compilation-output code=\"%d\">\n%s\n</compilation-output>\n",
			slice.CommitGraphNode.CompilationResult.ExitCode, stripEndNewline(stripLeanOutput(slice.CommitGraphNode.CompilationResult.Out, leanOutputParams))))
	}

	sb.WriteString("Assistant's next step:\n")

	return InferenceTask{
		Prompt: sb.String(),
	}, nil
}

func (rg *RepoGraph) BuildCompilationTasksForNode(nodeLocator NodeLocator) (CompilationTask, error) {
	slice, err := rg.GetNodeSlice(nodeLocator)
	if err != nil {
		return CompilationTask{}, err
	}
	node := slice.CommitGraphNode
	if node.IsRoot() {
		return CompilationTask{}, fmt.Errorf("tried to compile the root node %v", node.ID)
	}
	parent := slice.CommitGraph.Nodes[*node.Parent]
	preCommands := []CompilationPreCommand{}
	parsedAction, err := ParseModelResponse(node.InferenceOutput)
	if err != nil {
		return CompilationTask{}, fmt.Errorf("Should not happen: node %v has non-parsable inference output %w", node.ID, err)
	}
	for _, action := range parsedAction.Actions.Items {
		task := action.GetCompilationTask()
		if task != "" {
			preCommands = append(preCommands, CompilationPreCommand{
				Name:   action.GetType(),
				Script: task,
			})
		} else if action.GetType() == "git-status" {
			// git status is special because the LLM doesn't know we are actually using
			// branches instead of building a single commit
			preCommands = append(preCommands, CompilationPreCommand{
				Name:   "git-status",
				Script: fmt.Sprintf("git diff origin/%s", slice.BranchTarget.BranchName),
			})
		}
	}

	preCommands = append(preCommands, CompilationPreCommand{
		Name:   "mk_all-hidden",
		Script: "lake exec mk_all --lib Corelib",
	})

	// helps remove noise from caching & building unrelated files
	preCommands = append(preCommands, CompilationPreCommand{
		Name:   "prebuild-hidden",
		Script: "lake build",
	})

	parsedActionsLength := len(parsedAction.Actions.Items)
	if parsedActionsLength > 0 && parsedAction.Actions.Items[parsedActionsLength-1].GetType() == "git-commit" {
		// git commit is special in many ways. When the LLM is ready to commit, we take a snapshot of the diff
		// between the root of the BT and the current branch. That diff is the diffPatch that allows us to dedupe
		// identical nodes within the commit graph.
		//
		// We could also use git ls-files | xargs cat | sha256sum to get a unique id (or something similar).
		// However, this serves a dual purpose and allows us to easily review & understand branch differences
		// without having to inspect the graph
		//
		// git-commit also runs after mk_all-hidden because that will produce a diff
		preCommands = append(preCommands, CompilationPreCommand{
			// -hidden not needed because the model will never execute past this point.
			Name:   "git-commit",
			Script: fmt.Sprintf("git diff --minimal origin/%s", slice.BranchTarget.BranchName),
		})
	}

	return CompilationTask{
		BranchName: parent.BranchName,
		// compile onto the new branch
		NewBranchName:     node.BranchName,
		PreCommands:       preCommands,
		CompilationScript: "lake build",
	}, nil
}

/**
 * λ  = decay rate (a higher value means we will prioritize less-seen branches) (>0)
 * α  = grace period (a lower value means more early exploration) (0-1)
 * β  = inProgress penalty multiplier (higher values mean less work will be
 *                 scheduled simultaneously) (>0)
 *
 *          succ(bt) + 1                             1
 * w(bt) =  ------------ * -------------------------------------------------------------
 *          fail(bt) + 1   inProgress(bt) * β + ( 1 +  α * (fail(bt) + succ(bt)) )^( 1 + λ )
 *
 * This came to me during a walk in central park.
 * It feels about right. But I should probably do some research.
 * https://www.desmos.com/calculator/uu9zk9fnhd
 *
 * It also might be interesting to experiment with graph depth as a param.
 * I hope that deeper graphs will have higher success rates... but not sure.
 * Newer graphs will have a higher weight, so perhaps that is enough
 */
func (bt *RepoGraphBranchTarget) RandomSamplingWeight() float64 {
	lambda := 0.6
	alpha := 0.2
	beta := 0.5
	numInProgress := float64(0)
	numSuccess := float64(0)
	numFail := float64(0)
	for _, subgraph := range bt.Subgraphs {
		if subgraph.State == GraphStateInProgress || subgraph.State == GraphStateAwaitingGoalSetup {
			numInProgress++
		}
		if subgraph.State == GraphStateSuccess {
			numSuccess++
		}
		// Don't include failed setup graphs... I don't understand when they happen yet.
		if subgraph.State == GraphStateFailed {
			numFail++
		}
	}
	result := (numSuccess + 1) / (numFail + 1)
	result = result * (1 / (numInProgress*beta + math.Pow(1+alpha*(numFail+numSuccess), 1+lambda)))
	return result
}

func (gn *CommitGraphNode) IsRoot() bool {
	return gn.Parent == nil
}

func (rg *RepoGraph) CountBranchTargetsWithGoal(goalID GoalID) int {
	count := 0
	for _, branchTarget := range rg.BranchTargets {
		if branchTarget.Subgraphs[goalID] != nil {
			count++
		}
	}
	return count
}

// check if any of the parents of a branch target are created by the goal
// all goals should be cummulative and commute but should not be applied twice
//
// THIS DOES NOT CHECK IF THE CURRENT BRANCH CONTAINS A CHILD GOAL ID.
// Use bt.Subgraphs[goalID] != nil for that.
func (rg *RepoGraph) BranchTargetDerivesFromGoal(branchTarget *RepoGraphBranchTarget, goalID GoalID) bool {
	bt := branchTarget
	for bt.ParentBranchName != nil {
		if bt.TraversalGoalID != nil && *bt.TraversalGoalID == goalID {
			return true
		}
		bt = rg.BranchTargets[*bt.ParentBranchName]
	}
	return false
}

func (rg *RepoGraph) BranchTargetDepth(branchTarget *RepoGraphBranchTarget) int {
	depth := 0
	for branchTarget.ParentBranchName != nil {
		branchTarget = rg.BranchTargets[*branchTarget.ParentBranchName]
		depth++
	}
	return depth
}

func (rg *RepoGraph) FindNewBranchTargetForGoal(goalID GoalID) *RepoGraphBranchTarget {
	// TODO: See if this is worth caching (and how to do it)
	choices := []weightedrand.Choice[*RepoGraphBranchTarget, int64]{}
	for _, branchTarget := range rg.BranchTargets {
		if branchTarget.Subgraphs[goalID] != nil || rg.BranchTargetDerivesFromGoal(branchTarget, goalID) {
			continue
		}
		choices = append(choices,
			weightedrand.NewChoice(branchTarget, int64(100000*rg.BranchTargetDepth(branchTarget))+int64(1000*branchTarget.RandomSamplingWeight())))
	}
	if len(choices) == 0 {
		return nil
	}
	chooser, err := weightedrand.NewChooser(choices...)
	if err != nil {
		return nil
	}
	return chooser.Pick()
}

func CreateGraphCreateCli() *cli.Command {
	var rootBranchName string
	var path string
	action := func(ctx context.Context, _ *cli.Command) error {
		rg := NewRepoGraph(BranchName(rootBranchName))
		rg.SaveToFile(path)
		return nil
	}
	return &cli.Command{
		Name:   "graph-create",
		Usage:  "create an empty graph",
		Action: action,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "root-branch",
				Usage:       "root branch name",
				Destination: &rootBranchName,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "out",
				Usage:       "path to save the graph",
				Destination: &path,
				Required:    true,
			},
		},
	}
}
