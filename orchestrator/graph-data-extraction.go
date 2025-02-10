package main

import (
	"math"
)

type WeightedOutputData struct {
	NodeID           NodeID  `json:"node_id"`
	Output           string  `json:"output"`
	Advantage        float64 `json:"advantage"`
	RawReward        float64 `json:"raw_reward"`
	NormalizedReward float64 `json:"normalized_reward"`
}

type CommitGraphNodeData struct {
	NodeID  NodeID                `json:"node_id"`
	Prompt  string                `json:"prompt"`
	Outputs []*WeightedOutputData `json:"outputs"`
}

type CommitGraphData struct {
	// only map because it brings down backprop from O(n^2)
	Nodes map[NodeID]*CommitGraphNodeData `json:"nodes"`
}

// This is roughly inspired by https://arxiv.org/pdf/2402.03300 section 4.1.3:
// Process Supervision RL with GRPO
//
// (though written in a terribly convoluted way)
func (rg *RepoGraph) ExtractData(cgLocator CommitGraphLocator, goalProvider GoalProvider) (*CommitGraphData, error) {
	nodes := map[NodeID]*CommitGraphNodeData{}
	slice, err := rg.GetCommitGraphSlice(cgLocator)
	if err != nil {
		return nil, err
	}
	cg := slice.CommitGraph
	// it is not possible to back-prop the reward if the graph is not finished.
	// (we could do maximally-complete sub-trees but that more complexity than it is worth... probably)
	// TODO: revisit this once we have a value model that more-efficiently allocates compute in the CG traversal
	if cg.State != GraphStateSuccess {
		return &CommitGraphData{
			Nodes: nodes,
		}, nil
	}
	seen := map[NodeID]bool{}
	for _, parent := range cg.Nodes {
		// only build data for non-terminal nodes
		if len(parent.Children) == 0 {
			continue
		}
		outputs := []*WeightedOutputData{}
		for _, childId := range parent.Children {
			child := cg.Nodes[childId]
			reward := 0.0
			if child.Result == NodeResultSuccess {
				reward = 1.0
			}
			// proxy for terminal node (correct reward)
			if len(child.Children) == 0 {
				seen[childId] = true
			}
			//TODO: Consider penalizing for number of steps || tokens used.
			outputs = append(outputs, &WeightedOutputData{
				NodeID:           childId,
				Output:           child.InferenceOutput,
				Advantage:        0.0,
				RawReward:        reward,
				NormalizedReward: 0.0,
			})
		}
		task, err := rg.BuildInferenceTaskForNode(NodeLocator{
			CommitGraphLocator: cgLocator,
			NodeID:             parent.ID,
		}, goalProvider)
		if err != nil {
			return nil, err
		}
		nodes[parent.ID] = &CommitGraphNodeData{
			NodeID:  parent.ID,
			Prompt:  task.Prompt,
			Outputs: outputs,
		}
	}

	meanReward := 0.0
	for _, node := range nodes {
		for _, output := range node.Outputs {
			meanReward += output.RawReward
		}
	}
	meanReward /= float64(len(nodes))
	rewardVariance := 0.0
	for _, node := range nodes {
		for _, output := range node.Outputs {
			rewardVariance += math.Pow(output.RawReward-meanReward, 2)
		}
	}
	rewardVariance /= float64(len(nodes))
	rewardStdDev := math.Sqrt(rewardVariance)
	for _, node := range nodes {
		for _, output := range node.Outputs {
			// Unclear if we should penalize for simply existing (using compute).
			// Right now, this if statement ensures we only reward good nodes.
			// If we want to swap, change the continues in the if statements below.
			if output.RawReward == 0.0 {
				continue
			}
			if !seen[output.NodeID] {
				//continue
			}
			output.NormalizedReward = (output.RawReward - meanReward) / rewardStdDev
			output.Advantage = output.NormalizedReward
		}
	}
	numUpdated := 1
	// non-sorted backprop ~O(depth * num_nodes)
	for numUpdated > 0 {
		numUpdated = 0
		for _, datum := range nodes {
		nextNode:
			for _, output := range datum.Outputs {
				if seen[output.NodeID] {
					continue nextNode
				}
				outputChild := nodes[output.NodeID]
				sumOfAdvantages := 0.0
				// This feels wrong. I want to consult other papers.
				// I don't like that this is normalized before summing.
				for _, outputChild := range outputChild.Outputs {
					if !seen[outputChild.NodeID] {
						// don't backprop until the entire previous layer has been updated
						continue nextNode
					}
					sumOfAdvantages += outputChild.Advantage
				}
				output.Advantage = sumOfAdvantages
				seen[output.NodeID] = true
				numUpdated++
			}
		}
	}
	return &CommitGraphData{
		Nodes: nodes,
	}, nil
}
