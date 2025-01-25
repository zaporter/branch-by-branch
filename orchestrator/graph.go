package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

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
	BranchTargets map[BranchName]*RepoGraphBranchTarget `json:"branch_targets"`
}

type RepoGraphBranchTarget struct {
	// nil if this is the root
	ParentBranchName *BranchName `json:"parent_branch_name,omitempty"`
	// Goal that was used to create this branch target (nil if this is the root)
	TraversalGoalID *GoalID                 `json:"traversal_goal_id,omitempty"`
	BranchName      BranchName              `json:"branch_name"`
	Subgraphs       map[GoalID]*CommitGraph `json:"subgraphs"`
}

type CommitGraph struct {
	// goal_id is a unique identifier for the graph because
	// we will never start a new graph at the same target on the same goal
	GoalID   GoalID                      `json:"goal_id"`
	RootNode NodeID                      `json:"root_node"`
	Nodes    map[NodeID]*CommitGraphNode `json:"nodes"`
	State    GraphState                  `json:"state"`
}

type CommitGraphNode struct {
	ID    NodeID `json:"id"`
	Depth int    `json:"depth"`
	// nil if this is the root
	Parent   *NodeID    `json:"parent"`
	Children []NodeID   `json:"children"`
	State    NodeState  `json:"state"`
	Result   NodeResult `json:"result"`

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
		Depth:      0,
		Parent:     nil,
		Children:   []NodeID{},
		State:      NodeStateAwaitingGoalSetup,
		Result:     NodeResultNone,
		BranchName: NewBranchName(),
	}
	return &CommitGraph{
		GoalID:   goalID,
		RootNode: rootNode.ID,
		Nodes:    map[NodeID]*CommitGraphNode{rootNode.ID: rootNode},
		State:    GraphStateAwaitingGoalSetup,
	}
}

func NewRepoGraph(rootBranchName BranchName) *RepoGraph {
	rg := &RepoGraph{
		BranchTargets: map[BranchName]*RepoGraphBranchTarget{
			rootBranchName: {
				BranchName:       rootBranchName,
				ParentBranchName: nil,
				TraversalGoalID:  nil,
				Subgraphs:        map[GoalID]*CommitGraph{},
			},
		},
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
	return json.Unmarshal(str, rg)
}

func (rg *RepoGraph) AddBranchTarget(parentBranchName BranchName, traversalGoalID GoalID) {
	branchName := NewBranchName()
	rg.BranchTargets[branchName] = &RepoGraphBranchTarget{
		BranchName:       branchName,
		ParentBranchName: &parentBranchName,
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
	slice.CommitGraph.State = GraphStateInProgress
	return nil
}

func (rg *RepoGraph) HandleInferenceOutput(locator NodeLocator, result *InferenceTaskResponse) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
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

		newNode := &CommitGraphNode{
			ID:              NewNodeID(),
			Depth:           node.Depth + 1,
			Parent:          &node.ID,
			Children:        []NodeID{},
			State:           NodeStateAwaitingCompilation,
			Result:          NodeResultNone,
			InferenceOutput: seq,
			ActionOutputs:   []ActionOutput{},
			BranchName:      NewBranchName(),
		}
		// check for syntax errors
		// (easier here before pulling this off to queue)
		parsed, err := ParseModelResponse(seq)
		if err != nil || parsed.Actions.Validate() != nil {
			newNode.State = NodeStateDone
			newNode.Result = NodeResultSyntaxFailure
		}
		slice.CommitGraph.Nodes[newNode.ID] = newNode
		node.Children = append(node.Children, newNode.ID)
	}
	return nil
}

func (rg *RepoGraph) HandleCompilationOutput(locator NodeLocator, result *CompilationTaskResponse, maxDepth int) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	if slice.CommitGraphNode.State != NodeStateRunningCompilation {
		return fmt.Errorf("node %v is not in state RunningCompilation", locator)
	}
	if slice.CommitGraph.State != GraphStateInProgress {
		return fmt.Errorf("graph %v is not in state InProgress", locator)
	}
	node := slice.CommitGraphNode

	resp, err := ParseModelResponse(node.InferenceOutput)
	if err != nil {
		return fmt.Errorf("node somehow got compiled with non-parsable inference output %v %w", locator, err)
	}
	for _, res := range result.PreCommandsResults {
		node.ActionOutputs = append(node.ActionOutputs, ActionOutput{
			ActionName: res.ActionName,
			Text:       res.Out,
			ExitCode:   res.ExitCode,
		})
	}
	node.CompilationResult = &result.CompilationResult

	if resp.Actions.ContainsGitCommit() {
		node.State = NodeStateDone
		if node.CompilationResult.ExitCode == 0 {
			node.Result = NodeResultSuccess
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

	// If all nodes are done, we can determine if the graph is successful
	if len(slice.CommitGraph.Nodes) == len(slice.CommitGraph.AllNodesInState(NodeStateDone)) {
		hasSuccess := false
		for _, node := range slice.CommitGraph.Nodes {
			if node.Result == NodeResultSuccess {
				hasSuccess = true
			}
		}
		if hasSuccess {
			// ❤️ create a new branch target!
			slice.CommitGraph.State = GraphStateSuccess
			rg.AddBranchTarget(slice.BranchTarget.BranchName, slice.CommitGraph.GoalID)
		} else {
			slice.CommitGraph.State = GraphStateFailed
		}
	}

	return nil
}

func (rg *RepoGraph) UnfinishedGraphs() []CommitGraphLocator {
	unfinishedGraphs := []CommitGraphLocator{}
	for branchName, branchTarget := range rg.BranchTargets {
		for problemID, subgraph := range branchTarget.Subgraphs {
			if subgraph.State == GraphStateInProgress {
				unfinishedGraphs = append(unfinishedGraphs, CommitGraphLocator{BranchTarget: branchName, ProblemID: problemID})
			}
		}
	}
	return unfinishedGraphs
}

// Traverse all the branch targets, usiing weighting, select a problem, select a branch
// Create a new CommitGraph, set the root node, and return the locator
// returns false if no new graphs can be spawned
func (rg *RepoGraph) SpawnNewGraph() (*CommitGraphLocator, bool) {
	//TODO
	return nil, false
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

func (rg *RepoGraph) BuildInferenceTaskForNode(nodeLocator NodeLocator) (InferenceTask, error) {
	_, err := rg.GetNodeSlice(nodeLocator)
	if err != nil {
		return InferenceTask{}, err
	}
	return InferenceTask{
		Prompt: "Tell me the weather in San Francisco",
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
				Script: fmt.Sprintf("git diff %s", slice.BranchTarget.BranchName),
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

func (rg *RepoGraph) FindNewBranchTargetForGoal(goalID GoalID) *RepoGraphBranchTarget {
	// TODO: See if this is worth caching (and how to do it)
	choices := []weightedrand.Choice[*RepoGraphBranchTarget, int64]{}
	for _, branchTarget := range rg.BranchTargets {
		if branchTarget.Subgraphs[goalID] != nil || rg.BranchTargetDerivesFromGoal(branchTarget, goalID) {
			continue
		}
		choices = append(choices,
			weightedrand.NewChoice(branchTarget, int64(100000*branchTarget.RandomSamplingWeight())))
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

func createGraphCreateCli() *cli.Command {
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
