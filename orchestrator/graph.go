package main

import "errors"

type NodeState string

const (
	// Needs to be scheduled to go through action-execution.go
	// From here, the node will either go to:
	// - awaiting_inference (if no filesystem changes were made)
	// - awaiting_compilation (if we need to run the compiler (new branch or filesystem changes))
	NodeStateAwaitingActionEvaluation NodeState = "node_awaiting_action_evaluation"
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
)

type CommitGraphLocator struct {
	BranchName BranchName
	ProblemID  GoalID
}

type NodeLocator struct {
	BranchName BranchName
	ProblemID  GoalID
	NodeID     NodeID
}

// NodeSlice is a slice through the repo down to a CommitGraphNode
type NodeSlice struct {
	BranchTarget    *RepoGraphBranchTarget
	CommitGraph     *CommitGraph
	CommitGraphNode *CommitGraphNode
}

// CommitGraphSlice is a slice through the repo down to a CommitGraph
type CommitGraphSlice struct {
	BranchTarget *RepoGraphBranchTarget
	CommitGraph  *CommitGraph
}

type RepoGraph struct {
	BranchTargets map[BranchName]RepoGraphBranchTarget `json:"branch_targets"`
}

// weighting algo is ((n_succ)/(n_fail+1))/(n_attempt^(\lambda+1))
type RepoGraphBranchTarget struct {
	BranchName BranchName             `json:"branch_name"`
	Subgraphs  map[GoalID]CommitGraph `json:"subgraphs"`
}

type CommitGraph struct {
	// goal_id is a unique identifier for the graph because
	// we will never start a new graph at the same target on the same goal
	GoalID   GoalID                     `json:"goal_id"`
	RootNode NodeID                     `json:"root_node"`
	Nodes    map[NodeID]CommitGraphNode `json:"nodes"`
	State    GraphState                 `json:"state"`
	// apply(goals[goal_id].GoalStatement @ parent.branch_name) is written to branch_name
	BranchName BranchName `json:"branch_name"`
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
	InferenceOutput string `json:"inference_output"`
	// The results of the computation from apply(parse(inference_output) @ parent.branch_name)
	ActionOutputs []ActionOutput `json:"action_outputs"`
	// apply(parse(inference_output) @ parent.branch_name) is written to branch_name
	BranchName BranchName `json:"branch_name"`
}

// not an action, but a way to return output from an action
type ActionOutput struct {
	ActionName string `json:"action_name"`
	Text       string `json:"text"`
}

func (gn *CommitGraphNode) IsRoot() bool {
	return gn.Parent == nil
}

func NewCommitGraphRoot() CommitGraph {
	return CommitGraph{}
}

type GoalI interface {
	ID() GoalID
	GoalStatement() string
	// Returns a setup script that will be run on the branch
	SetupOnBranch(BranchName) CompilationTask
	// Returns true if the setup script was successful
	// & if the branch is ready to be explored
	//
	// If this returns false, the branch should not be scheduled.
	ValidateSetup(CompilationResult) bool
}

func NewRepoGraph(rootBranchName BranchName) *RepoGraph {
	rg := &RepoGraph{
		BranchTargets: make(map[BranchName]RepoGraphBranchTarget),
	}
	rg.AddBranchTarget(rootBranchName)
	return rg
}

func (rg *RepoGraph) AddBranchTarget(branchName BranchName) {
	rg.BranchTargets[branchName] = RepoGraphBranchTarget{
		BranchName: branchName,
		Subgraphs:  make(map[GoalID]CommitGraph),
	}
}

func (rg *RepoGraph) HandleInferenceOutput(locator NodeLocator, result *InferenceTaskResponse) error {
	tree, err := rg.GetNodeSlice(locator)
	if err != nil {
		return err
	}
	node := tree.CommitGraphNode
	node.State = NodeStateDone
	// if we have children, we are (by definition) non-terminal
	node.Result = NodeResultNone
	for _, seq := range result.ReturnSequences {
		newNode := CommitGraphNode{
			ID:              NewNodeID(),
			Depth:           node.Depth + 1,
			Parent:          &node.ID,
			Children:        []NodeID{},
			State:           NodeStateAwaitingActionEvaluation,
			Result:          NodeResultNone,
			InferenceOutput: seq,
			ActionOutputs:   []ActionOutput{},
		}
		tree.CommitGraph.Nodes[newNode.ID] = newNode
		node.Children = append(node.Children, newNode.ID)
	}
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

func (cg *CommitGraph) AllNodesInState(state NodeState) []CommitGraphNode {
	nodes := []CommitGraphNode{}
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

func (rg *RepoGraph) GetNodeSlice(locator NodeLocator) (NodeSlice, error) {
	branchTarget, ok := rg.BranchTargets[locator.BranchName]
	if !ok {
		return NodeSlice{}, errors.New("branch target not found")
	}
	subgraph, ok := branchTarget.Subgraphs[locator.ProblemID]
	if !ok {
		return NodeSlice{}, errors.New("subgraph not found")
	}
	node, ok := subgraph.Nodes[locator.NodeID]
	if !ok {
		return NodeSlice{}, errors.New("node not found")
	}
	return NodeSlice{
		BranchTarget:    &branchTarget,
		CommitGraph:     &subgraph,
		CommitGraphNode: &node,
	}, nil
}

func (rg *RepoGraph) GetCommitGraphSlice(locator CommitGraphLocator) (CommitGraphSlice, error) {
	branchTarget, ok := rg.BranchTargets[locator.BranchName]
	if !ok {
		return CommitGraphSlice{}, errors.New("branch target not found")
	}
	subgraph, ok := branchTarget.Subgraphs[locator.ProblemID]
	if !ok {
		return CommitGraphSlice{}, errors.New("subgraph not found")
	}
	return CommitGraphSlice{
		BranchTarget: &branchTarget,
		CommitGraph:  &subgraph,
	}, nil
}
