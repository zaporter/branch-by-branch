package main

import "errors"

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
		BranchTarget:    branchTarget,
		CommitGraph:     subgraph,
		CommitGraphNode: node,
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
		BranchTarget: branchTarget,
		CommitGraph:  subgraph,
	}, nil
}
