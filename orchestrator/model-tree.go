package orchestrator

import (
	"encoding/json"
	"os"
)

// TODO: THIS IS UNUSED. I WILL WANT THIS TO FIX THE TRAINING LOOP ISSUE.

// Suprise! Its another 🌲!
// Luckily, the tree nodes will almost always have len(Children) <=1.
// However, I am building it this way because I can totally see myself wanting the ability to
// represent the idea of trashing a batch of data and then resetting to a prior model.
// (in the event of auto-detected model collapse)
type ModelTree struct {
	Root  ModelTreeNodeID
	Nodes map[ModelTreeNodeID]*ModelTreeNode
}

type ModelTreeNode struct {
	ModelTreeNodeID ModelTreeNodeID
	Children        []ModelTreeNodeID
	ModelName       string
	AdapterName     string
}

func NewModelTree(rootModelName string, rootAdapterName string) *ModelTree {
	rootNodeID := NewModelTreeNodeID(rootModelName, rootAdapterName)
	rootNode := &ModelTreeNode{
		ModelTreeNodeID: rootNodeID,
		Children:        []ModelTreeNodeID{},
		ModelName:       rootModelName,
		AdapterName:     rootAdapterName,
	}
	return &ModelTree{
		Root:  rootNodeID,
		Nodes: map[ModelTreeNodeID]*ModelTreeNode{rootNodeID: rootNode},
	}
}

func (mt *ModelTree) AddNode(node *ModelTreeNode) {
	mt.Nodes[node.ModelTreeNodeID] = node
}

func (mt *ModelTree) SaveToFile(path string) error {
	bytes, err := json.Marshal(mt)
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

func (mt *ModelTree) LoadFromFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, mt)
}
