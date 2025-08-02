package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGraphDataExtraction_SingleNode(t *testing.T) {
	rg := NewRepoGraph(BranchName("test"))
	cg := NewCommitGraph(GoalID("goal_id"))
	rg.BranchTargets[BranchName("test")].Subgraphs[GoalID("goal_id")] = cg
	_, err := rg.AddNodeToCommitGraph(NodeLocator{
		CommitGraphLocator: CommitGraphLocator{
			BranchTargetLocator: BranchTargetLocator{
				BranchName: BranchName("test"),
			},
			GoalID: GoalID("goal_id"),
		},
		NodeID: cg.RootNode,
	}, "test", NodeMetadata{})
	assert.Nil(t, err)
	_, err = rg.ExtractData(CommitGraphLocator{
		BranchTargetLocator: BranchTargetLocator{
			BranchName: BranchName("test"),
		},
		GoalID: GoalID("goal_id"),
	}, nil)
	assert.Nil(t, err)

}
