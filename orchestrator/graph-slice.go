package main

import "errors"

type BranchTargetLocator struct {
	BranchName BranchName `json:"branch_name"`
}

type CommitGraphLocator struct {
	BranchTargetLocator BranchTargetLocator `json:"branch_target_locator"`
	GoalID              GoalID              `json:"goal_id"`
}

type NodeLocator struct {
	CommitGraphLocator CommitGraphLocator `json:"commit_graph_locator"`
	NodeID             NodeID             `json:"node_id"`
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

func (rg *RepoGraph) GetNodeSlice(locator NodeLocator) (NodeSlice, error) {
	branchTarget, ok := rg.BranchTargets[locator.CommitGraphLocator.BranchTargetLocator.BranchName]
	if !ok {
		return NodeSlice{}, errors.New("branch target not found")
	}
	subgraph, ok := branchTarget.Subgraphs[locator.CommitGraphLocator.GoalID]
	if !ok {
		return NodeSlice{}, errors.New("subgraph not found")
	}
	node, ok := subgraph.Nodes[locator.NodeID]
	if !ok {
		return NodeSlice{}, errors.New("node not found")
	}
	return NodeSlice{
		BranchTarget:    branchTarget,
		CommitGraph:     subgraph,
		CommitGraphNode: node,
	}, nil
}

func (rg *RepoGraph) GetCommitGraphSlice(locator CommitGraphLocator) (CommitGraphSlice, error) {
	branchTarget, ok := rg.BranchTargets[locator.BranchTargetLocator.BranchName]
	if !ok {
		return CommitGraphSlice{}, errors.New("branch target not found")
	}
	subgraph, ok := branchTarget.Subgraphs[locator.GoalID]
	if !ok {
		return CommitGraphSlice{}, errors.New("subgraph not found")
	}
	return CommitGraphSlice{
		BranchTarget: branchTarget,
		CommitGraph:  subgraph,
	}, nil
}
