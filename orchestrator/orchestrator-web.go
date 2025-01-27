package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func setupHeader(w *http.ResponseWriter, isJson bool) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if isJson {
		(*w).Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		(*w).Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func (o *Orchestrator) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, false)
		w.Write([]byte("pong"))
	})

	mux.HandleFunc("/api/graph/branch-target-graph-locators", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		o.mu.Lock()
		defer o.mu.Unlock()
		type GoalBranchNode struct {
			ParentBranchTarget    BranchTargetLocator   `json:"parent_branch_target"`
			ChildrenBranchTargets []BranchTargetLocator `json:"children_branch_targets"`
			CommitGraph           CommitGraphLocator    `json:"commit_graph"`
			GoalName              string                `json:"goal_name"`
		}
		type Response struct {
			BranchTargets []BranchTargetLocator `json:"branch_targets"`
			Subgraphs     []GoalBranchNode      `json:"subgraphs"`
		}
		response := Response{}
		for _, bt := range o.RepoGraph.BranchTargets {
			response.BranchTargets = append(response.BranchTargets, BranchTargetLocator{
				BranchName: bt.BranchName,
			})
			for _, subgraph := range bt.Subgraphs {
				children := []BranchTargetLocator{}
				// N^2
				for _, testB := range o.RepoGraph.BranchTargets {
					if testB.TraversalGoalID != nil && *testB.TraversalGoalID == subgraph.GoalID {
						children = append(children, BranchTargetLocator{
							BranchName: testB.BranchName,
						})
					}
				}
				name := o.GoalProvider.GetGoal(subgraph.GoalID).Name()

				response.Subgraphs = append(response.Subgraphs, GoalBranchNode{
					ParentBranchTarget: BranchTargetLocator{
						BranchName: bt.BranchName,
					},
					ChildrenBranchTargets: children,
					CommitGraph: CommitGraphLocator{
						BranchTargetLocator: BranchTargetLocator{
							BranchName: bt.BranchName,
						},
						GoalID: subgraph.GoalID,
					},
					GoalName: name,
				})
			}
		}
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/graph/commit-graph-locators", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request CommitGraphLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		slice, err := o.RepoGraph.GetCommitGraphSlice(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type CommitGraphLocatorsNode struct {
			Locator  NodeLocator   `json:"locator"`
			Result   NodeResult    `json:"result"`
			State    NodeState     `json:"state"`
			Depth    int           `json:"depth"`
			Children []NodeLocator `json:"children"`
		}
		type Response struct {
			State GraphState `json:"state"`
			// over the api, it is nicer to return a locator
			// (even though it is a lot of redundant bytes)
			RootNode NodeLocator               `json:"root_node"`
			Nodes    []CommitGraphLocatorsNode `json:"nodes"`
		}
		nodes := []CommitGraphLocatorsNode{}
		for _, node := range slice.CommitGraph.Nodes {
			children := []NodeLocator{}
			for _, child := range node.Children {
				children = append(children, NodeLocator{
					CommitGraphLocator: request,
					NodeID:             child,
				})
			}
			nodes = append(nodes, CommitGraphLocatorsNode{
				Locator: NodeLocator{
					CommitGraphLocator: request,
					NodeID:             node.ID,
				},
				Result:   node.Result,
				State:    node.State,
				Depth:    node.Depth,
				Children: children,
			})
		}
		response := Response{
			State: slice.CommitGraph.State,
			RootNode: NodeLocator{
				CommitGraphLocator: request,
				NodeID:             slice.CommitGraph.RootNode,
			},
			Nodes: nodes,
		}
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/graph/branch-target-stats", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request BranchTargetLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		type Response struct {
			BranchName       BranchName  `json:"branch_name"`
			ParentBranchName *BranchName `json:"parent_branch_name,omitempty"`
			NumSubgraphs     int         `json:"num_subgraphs"`
		}
		slice, err := o.RepoGraph.GetBranchTargetSlice(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response := Response{
			BranchName:       slice.BranchTarget.BranchName,
			ParentBranchName: slice.BranchTarget.ParentBranchName,
			NumSubgraphs:     len(slice.BranchTarget.Subgraphs),
		}
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/graph/commit-graph-stats", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request CommitGraphLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		type Response struct {
			State  GraphState `json:"state"`
			GoalID GoalID     `json:"goal_id"`
		}
		slice, err := o.RepoGraph.GetCommitGraphSlice(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response := Response{
			State:  slice.CommitGraph.State,
			GoalID: slice.CommitGraph.GoalID,
		}
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/graph/node-stats", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request NodeLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		type Response struct {
			Depth             int                `json:"depth"`
			State             NodeState          `json:"state"`
			Result            NodeResult         `json:"result"`
			InferenceOutput   string             `json:"inference_output"`
			BranchName        BranchName         `json:"branch_name"`
			ActionOutputs     []ActionOutput     `json:"action_outputs"`
			CompilationResult *CompilationResult `json:"compilation_result,omitempty"`
			Prompt            string             `json:"prompt,omitempty"`
		}
		slice, err := o.RepoGraph.GetNodeSlice(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prompt_task, err := o.RepoGraph.BuildInferenceTaskForNode(request, o.GoalProvider)
		var prompt string
		if err != nil {
			prompt = fmt.Sprintf("Error building prompt: %s", err.Error())
		} else {
			prompt = prompt_task.Prompt
		}
		response := Response{
			Depth:             slice.CommitGraphNode.Depth,
			State:             slice.CommitGraphNode.State,
			Result:            slice.CommitGraphNode.Result,
			InferenceOutput:   slice.CommitGraphNode.InferenceOutput,
			BranchName:        slice.CommitGraphNode.BranchName,
			ActionOutputs:     slice.CommitGraphNode.ActionOutputs,
			CompilationResult: slice.CommitGraphNode.CompilationResult,
			Prompt:            prompt,
		}
		json.NewEncoder(w).Encode(response)
	})

}
