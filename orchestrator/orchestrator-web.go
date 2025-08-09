package orchestrator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
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
				// TODO: refactor this to use CG.Results.
				// N^2
				for _, testB := range o.RepoGraph.BranchTargets {
					if testB.TraversalGoalID != nil &&
						*testB.TraversalGoalID == subgraph.GoalID &&
						testB.ParentBranchName != nil &&
						*testB.ParentBranchName == bt.BranchName {
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
			Locator              NodeLocator   `json:"locator"`
			Result               NodeResult    `json:"result"`
			State                NodeState     `json:"state"`
			Depth                int           `json:"depth"`
			Metadata             NodeMetadata  `json:"metadata"`
			TerminationRequested bool          `json:"termination_requested"`
			Children             []NodeLocator `json:"children"`
			ChildrenAdvantages   []float64     `json:"children_advantages"`
		}
		type Response struct {
			State GraphState `json:"state"`
			// over the api, it is nicer to return a locator
			// (even though it is a lot of redundant bytes)
			RootNode NodeLocator               `json:"root_node"`
			Nodes    []CommitGraphLocatorsNode `json:"nodes"`
		}
		advantageData, err := o.RepoGraph.ExtractData(request, o.GoalProvider)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		nodes := []CommitGraphLocatorsNode{}
		for _, node := range slice.CommitGraph.Nodes {
			children := []NodeLocator{}
			childrenAdvantages := []float64{}
			advantageForNode := advantageData.Nodes[node.ID]
			for _, child := range node.Children {
				children = append(children, NodeLocator{
					CommitGraphLocator: request,
					NodeID:             child,
				})
				if advantageForNode != nil {
					for _, output := range advantageForNode.Outputs {
						if output.NodeID == child {
							childrenAdvantages = append(childrenAdvantages, output.Advantage)
						}
					}
				}
			}
			nodes = append(nodes, CommitGraphLocatorsNode{
				Locator: NodeLocator{
					CommitGraphLocator: request,
					NodeID:             node.ID,
				},
				Result:               node.Result,
				State:                node.State,
				Depth:                node.Depth,
				TerminationRequested: node.TerminationRequested,
				Metadata:             node.Metadata,
				Children:             children,
				ChildrenAdvantages:   childrenAdvantages,
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

	mux.HandleFunc("/api/graph/set-commit-graph-state", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		type Request struct {
			CommitGraphLocator CommitGraphLocator `json:"commit_graph_locator"`
			State              GraphState         `json:"state"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		slice, err := o.RepoGraph.GetCommitGraphSlice(request.CommitGraphLocator)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.logger.Info().Msgf("setting commit graph state to %s from %s", request.State, slice.CommitGraph.State)
		slice.CommitGraph.State = request.State
		w.Write([]byte("{}"))
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
			Depth                int                `json:"depth"`
			State                NodeState          `json:"state"`
			Result               NodeResult         `json:"result"`
			InferenceOutput      string             `json:"inference_output"`
			BranchName           BranchName         `json:"branch_name"`
			ActionOutputs        []ActionOutput     `json:"action_outputs"`
			Metadata             NodeMetadata       `json:"metadata"`
			TerminationRequested bool               `json:"termination_requested"`
			CompilationResult    *CompilationResult `json:"compilation_result,omitempty"`
			Prompt               string             `json:"prompt,omitempty"`
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
			Depth:                slice.CommitGraphNode.Depth,
			State:                slice.CommitGraphNode.State,
			Result:               slice.CommitGraphNode.Result,
			InferenceOutput:      slice.CommitGraphNode.InferenceOutput,
			BranchName:           slice.CommitGraphNode.BranchName,
			ActionOutputs:        slice.CommitGraphNode.ActionOutputs,
			CompilationResult:    slice.CommitGraphNode.CompilationResult,
			Prompt:               prompt,
			Metadata:             slice.CommitGraphNode.Metadata,
			TerminationRequested: slice.CommitGraphNode.TerminationRequested,
		}
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/graph/request-node-termination", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request NodeLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		err = o.RepoGraph.RequestNodeTerminationRecursively(request, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// todo: return some kind of status
		w.Write([]byte("{}"))
	})
	// THIS IS AN EXTREMELY DANGEROUS OPERATION THAT CAN CRASH THE ORCHESTRATOR
	// It should only ever be used on terminal nodes that are not generating new nodes
	// This does not recursively delete children. That is the operator's responsibility
	mux.HandleFunc("/api/graph/delete-node", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		var request NodeLocator
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		slice, err := o.RepoGraph.GetNodeSlice(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, node := range slice.CommitGraph.Nodes {
			if slices.Contains(node.Children, request.NodeID) {
				children := slices.DeleteFunc(node.Children, func(child NodeID) bool {
					return child == request.NodeID
				})
				node.Children = children
			}
		}
		delete(slice.CommitGraph.Nodes, request.NodeID)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/api/graph/create-node", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		type Request struct {
			ParentNodeLocator NodeLocator `json:"parent_node_locator"`
			InferenceOutput   string      `json:"inference_output"`
		}
		type Response struct {
			NodeLocator NodeLocator `json:"node_locator"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		locator, err := o.RepoGraph.AddNodeToCommitGraph(
			request.ParentNodeLocator,
			request.InferenceOutput,
			NodeMetadata{
				WasManuallyCreated: true,
			},
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response := Response{
			NodeLocator: locator,
		}
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/graph/set-node-metadata", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		type Request struct {
			NodeLocator NodeLocator  `json:"node_locator"`
			Metadata    NodeMetadata `json:"metadata"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		slice, err := o.RepoGraph.GetNodeSlice(request.NodeLocator)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slice.CommitGraphNode.Metadata = request.Metadata
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/api/graph/save-golden-sample", func(w http.ResponseWriter, r *http.Request) {
		setupHeader(&w, true)
		type Request struct {
			NodeLocator NodeLocator `json:"node_locator"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		slice, err := o.RepoGraph.GetNodeSlice(request.NodeLocator)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		prompt_task, err := o.RepoGraph.BuildInferenceTaskForNode(request.NodeLocator, o.GoalProvider)
		var prompt string
		if err != nil {
			prompt = fmt.Sprintf("Error building prompt: %s", err.Error())
		} else {
			prompt = prompt_task.Prompt
		}
		goldenSample := GoldenSample{
			ID:          NewGoldenSampleID(),
			RepoID:      o.RepoGraph.ID,
			NodeLocator: request.NodeLocator,
			Type:        GoldenSampleTypeProofGeneration,
			Timestamp:   time.Now(),
			Prompt:      prompt,
			Completion:  slice.CommitGraphNode.InferenceOutput,
		}
		if err := goldenSample.Write(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slice.CommitGraphNode.Metadata.IsGoldenSample = true
		w.Write([]byte("{}"))
	})
}
