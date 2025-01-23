package main

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
	GraphStateInProgress NodeState = "graph_in_progress"
	GraphStateSuccess    NodeState = "graph_success"
	GraphStateFailed     NodeState = "graph_failed"
)

type RepoGraph struct {
	BranchTargets []RepoGraphBranchTarget `json:"branch_targets"`
}

// weighting algo is ((n_succ)/(n_fail+1))/(n_attempt^(\lambda+1))
type RepoGraphBranchTarget struct {
	BranchName BranchName    `json:"branch_name"`
	Subgraphs  []CommitGraph `json:"subgraphs"`
}

type CommitGraph struct {
	ProblemID string                     `json:"problem_id"`
	RootNode  NodeID                     `json:"root_node"`
	Nodes     map[NodeID]CommitGraphNode `json:"nodes"`
	State     GraphState                 `json:"state"`
}

type CommitGraphNode struct {
	ID    NodeID `json:"id"`
	Depth int    `json:"depth"`
	// nil if this is the root
	Parent   *NodeID    `json:"parent"`
	Children []NodeID   `json:"children"`
	State    NodeState  `json:"state"`
	Result   NodeResult `json:"result"`

	InferenceOutput string         `json:"inference_output"`
	ActionOutputs   []ActionOutput `json:"action_outputs"`
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

type ProblemI interface {
	ID() string
	ProblemGoal() string
	SetupOnBranch(branch string) error
}
