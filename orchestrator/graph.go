package main

import (
	"math"

	"github.com/rs/zerolog"
)

type NodeState string

const (
	// Will be queued to be compiled
	NodeStateAwaitingCompilation NodeState = "node_awaiting_compilation"
	// Will be queued to have inference run (producing children)
	NodeStateAwaitingInference NodeState = "node_awaiting_inference"
	// All nodes will rest here until the graph is finished
	// At that point, a reward can be calculated for each child
	// NodeResult should be calculated at this point (it may remain none for non-terminal nodes)
	NodeStateDone NodeState = "node_state_done"
)

type NodeResult string

const (
	// non-terminal node or in-progress
	NodeResultNone NodeResult = "node_result_none"
	// 🤙
	NodeResultSuccess NodeResult = "node_result_success"
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
	ParentBranchName *BranchName             `json:"parent_branch_name,omitempty"`
	BranchName       BranchName              `json:"branch_name"`
	Subgraphs        map[GoalID]*CommitGraph `json:"subgraphs"`
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
	CompilationOutput *CompilationTaskResponse `json:"compilation_output"`
	// apply(parse(inference_output) @ parent.branch_name) is written to branch_name
	// (unless this is the root, in which case, it is:
	// apply(goals[goal_id].GoalStatement @ branch_target.branch_name))
	BranchName BranchName `json:"branch_name"`
}

// not an action, but a way to return output from an action
type ActionOutput struct {
	ActionName string `json:"action_name"`
	Text       string `json:"text"`
}

func NewCommitGraph(goalID GoalID) CommitGraph {
	rootNode := &CommitGraphNode{
		ID:       NewNodeID(),
		Depth:    0,
		Parent:   nil,
		Children: []NodeID{},
		// Root is always done because computation
		// is blocked by the graph setup check
		State:      NodeStateDone,
		Result:     NodeResultNone,
		BranchName: NewBranchName(),
	}
	return CommitGraph{
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
				Subgraphs:        map[GoalID]*CommitGraph{},
			},
		},
	}
	return rg
}

func (rg *RepoGraph) AddBranchTarget(parentBranchName BranchName, branchName BranchName) {
	rg.BranchTargets[branchName] = &RepoGraphBranchTarget{
		BranchName:       branchName,
		ParentBranchName: &parentBranchName,
		Subgraphs:        map[GoalID]*CommitGraph{},
	}
}

func (rg *RepoGraph) HandleSetupCompilationOutput(logger *zerolog.Logger, locator NodeLocator, result *CompilationTaskResponse, goalProvider GoalProvider) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	goal := goalProvider.GetGoal(slice.CommitGraph.GoalID)
	ok := goal.ValidateSetup(*result)
	if !ok {
		logger.Error().Msgf("goal setup failed for %s on branch %s to branch %s", slice.CommitGraph.GoalID, slice.BranchTarget.BranchName, slice.CommitGraphNode.BranchName)
		slice.CommitGraph.State = GraphStateGoalSetupFailed
		return nil
	}
	slice.CommitGraphNode.State = NodeStateDone
	slice.CommitGraphNode.Result = NodeResultNone
	return nil
}

func (rg *RepoGraph) HandleInferenceOutput(locator NodeLocator, result *InferenceTaskResponse) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
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
		slice.CommitGraph.Nodes[newNode.ID] = newNode
		node.Children = append(node.Children, newNode.ID)
	}
	return nil
}
func (rg *RepoGraph) HandleCompilationOutput(locator NodeLocator, result *CompilationResult) error {
	slice, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	node := slice.CommitGraphNode
	node.State = NodeStateDone
	// if we have children, we are (by definition) non-terminal
	node.Result = NodeResultNone
	return nil
}

func (rg *RepoGraph) UnfinishedGraphs() []CommitGraphLocator {
	unfinishedGraphs := []CommitGraphLocator{}
	for branchName, branchTarget := range rg.BranchTargets {
		for problemID, subgraph := range branchTarget.Subgraphs {
			if subgraph.State == GraphStateInProgress {
				unfinishedGraphs = append(unfinishedGraphs, CommitGraphLocator{BranchName: branchName, ProblemID: problemID})
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

func (node *CommitGraphNode) GetPrompt() string {
	// TODO
	return "What is the weather in San Francisco?"
}

/**
 * λ  = decay rate (a higher value means less exploration) (0-1)
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
