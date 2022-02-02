package trees

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// JSONPrefixTree representation
type JSONPrefixTree struct {
	Root       []byte `json:"root"`
	IsComplete bool   `json:"isComplete"`
}

// JSONPrefixLeafNode representation
type JSONPrefixLeafNode struct {
	Hash          []byte `json:"hash"`
	Values        []byte `json:"values"`
	PartialPrefix []byte `json:"partialPrefix"`
}

// JSONPrefixInternalNode representation
type JSONPrefixInternalNode struct {
	Hash          []byte `json:"hash"`
	LeftChild     []byte `json:"leftChild"`
	RightChild    []byte `json:"rightChild"`
	PartialPrefix []byte `json:"partialPrefix"`
	LeftLeaf      bool   `json:"leftLeaf"`
	RightLeaf     bool   `json:"rightLeaf"`
}

// JSONPrefixInternalNode representation
type JSONSetCompletenessProofNode struct {
	Hash             []byte `json:"hash"`
	LeftChild        []byte `json:"leftChild"`
	RightChild       []byte `json:"rightChild"`
	PartialPrefix    []byte `json:"partialPrefix"`
	CumulativePrefix []byte `json:"cumulativePrefix"`
	IsLeft           bool   `json:"isLeft"`
}

// //*******************************
// // CORE METHODS
// //*******************************

// WriteBytesToFile writes a []byte to a given file
func WriteBytesToFile(buf []byte, path string) (int, error) {

	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}

	defer file.Close()

	n, err := file.Write(buf)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// ReadBytesFromFile reads all bytes from a given file
func ReadBytesFromFile(path string) ([]byte, error) {

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return ioutil.ReadAll(file)
}

func (PrefixTree *PrefixTree) Serialize() ([]byte, error) {

	tree, err := PrefixTree.root.serialize()
	if err != nil {
		return nil, err
	}

	jsonPrefix := JSONPrefixTree{
		tree,
		PrefixTree.isComplete,
	}

	b, err := json.Marshal(jsonPrefix)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (tree *PrefixTree) Deserialize(buf []byte) (*PrefixTree, error) {
	var jsonPrefix JSONPrefixTree
	err := json.Unmarshal(buf, &jsonPrefix)
	if err != nil {
		return nil, err
	}

	root, err := deserializePrefixInternalNode(jsonPrefix.Root)
	if err != nil {
		return nil, err
	}

	tree = &PrefixTree{
		root:       root,
		isComplete: jsonPrefix.IsComplete,
	}

	return tree, nil
}

// //*******************************
// // PREFIX NODE METHODS
// //*******************************

func (node *prefixLeafNode) serialize() ([]byte, error) {

	jsonLeaf := NewJSONPrefixLeaf(node)
	b, err := json.Marshal(jsonLeaf)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (node *prefixInternalNode) serialize() ([]byte, error) {

	var leftChild []byte
	var leftChildLeaf bool
	if node.getLeftChild() != nil {
		leftChild, _ = node.getLeftChild().serialize()
		leftChildLeaf = node.getLeftChild().isLeafNode()
	}

	var rightChild []byte
	var rightChildLeaf bool
	if node.getRightChild() != nil {
		rightChild, _ = node.getRightChild().serialize()
		rightChildLeaf = node.getRightChild().isLeafNode()
	}

	jsonInternal := NewJSONPrefixInternal(node, leftChild, rightChild, leftChildLeaf, rightChildLeaf)
	b, err := json.Marshal(jsonInternal)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (node *SetCompletenessProofNode) Serialize() ([]byte, error) {

	var leftChild []byte
	if node.GetLeftChild() != nil {
		leftChild, _ = node.GetLeftChild().Serialize()
	}

	var rightChild []byte
	if node.GetRightChild() != nil {
		rightChild, _ = node.GetRightChild().Serialize()
	}

	jsonInternal := NewJSONSetCompleteness(node, leftChild, rightChild, node.IsLeft)
	b, err := json.Marshal(jsonInternal)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func deserializePrefixLeafNode(buf []byte) (*prefixLeafNode, error) {

	var jsonLeaf JSONPrefixLeafNode
	err := json.Unmarshal(buf, &jsonLeaf)
	if err != nil {
		return &prefixLeafNode{}, err
	}

	var value []byte
	err = json.Unmarshal(jsonLeaf.Values, &value)
	if err != nil {
		return &prefixLeafNode{}, err
	}

	return &prefixLeafNode{
		hash:          jsonLeaf.Hash,
		value:         value,
		partialPrefix: jsonLeaf.PartialPrefix,
	}, nil
}

func deserializePrefixInternalNode(buf []byte) (*prefixInternalNode, error) {

	var jsonInternal JSONPrefixInternalNode
	err := json.Unmarshal(buf, &jsonInternal)

	if err != nil {
		return &prefixInternalNode{}, err
	}

	var leftChild prefixNode
	if jsonInternal.LeftChild != nil && jsonInternal.LeftLeaf {

		leftChild, err = deserializePrefixLeafNode(jsonInternal.LeftChild)
		if err != nil {
			return nil, err
		}
	} else if jsonInternal.LeftChild != nil && !jsonInternal.LeftLeaf {

		leftChild, err = deserializePrefixInternalNode(jsonInternal.LeftChild)
		if err != nil {
			return nil, err
		}
	}

	var rightChild prefixNode
	if jsonInternal.RightChild != nil && jsonInternal.RightLeaf {

		rightChild, err = deserializePrefixLeafNode(jsonInternal.RightChild)
		if err != nil {
			return nil, err
		}
	} else if jsonInternal.RightChild != nil && !jsonInternal.RightLeaf {
		rightChild, err = deserializePrefixInternalNode(jsonInternal.RightChild)
		if err != nil {
			return nil, err
		}
	}

	res := &prefixInternalNode{
		hash:          jsonInternal.Hash,
		leftChild:     leftChild,
		rightChild:    rightChild,
		partialPrefix: jsonInternal.PartialPrefix,
	}

	if leftChild != nil {
		leftChild.setParent(res)
	}

	if rightChild != nil {
		rightChild.setParent(res)
	}

	return res, nil
}

func DeserializeSetCompletenessProofNode(buf []byte) (*SetCompletenessProofNode, error) {

	var jsonSetCompleteness JSONSetCompletenessProofNode
	err := json.Unmarshal(buf, &jsonSetCompleteness)

	if err != nil {
		return &SetCompletenessProofNode{}, err
	}

	var leftChild *SetCompletenessProofNode
	if jsonSetCompleteness.LeftChild != nil {

		leftChild, err = DeserializeSetCompletenessProofNode(jsonSetCompleteness.LeftChild)
		if err != nil {
			return nil, err
		}
	}

	var rightChild *SetCompletenessProofNode
	if jsonSetCompleteness.RightChild != nil {

		rightChild, err = DeserializeSetCompletenessProofNode(jsonSetCompleteness.RightChild)
		if err != nil {
			return nil, err
		}
	}

	res := &SetCompletenessProofNode{
		CumulativePrefix: jsonSetCompleteness.CumulativePrefix,
		PartialPrefix:    jsonSetCompleteness.PartialPrefix,
		IsLeft:           jsonSetCompleteness.IsLeft,
		LeftChild:        leftChild,
		RightChild:       rightChild,
		Hash:             jsonSetCompleteness.Hash,
	}

	if leftChild != nil {
		leftChild.SetParent(res)
	}

	if rightChild != nil {
		rightChild.SetParent(res)
	}

	return res, nil
}

// NewJSONPrefixLeaf is a factory method for JSONPrefixLeaf structs
func NewJSONPrefixLeaf(node *prefixLeafNode) JSONPrefixLeafNode {

	values, _ := json.Marshal(node.getValue())

	return JSONPrefixLeafNode{
		node.getHash(),
		values,
		node.getPartialPrefix(),
	}
}

// NewJSONPrefixInternal is a factory method for JSONPrefixInternal structs
func NewJSONPrefixInternal(node *prefixInternalNode, leftChild []byte, rightChild []byte, left bool, right bool) JSONPrefixInternalNode {
	return JSONPrefixInternalNode{
		node.hash,
		leftChild,
		rightChild,
		node.getPartialPrefix(),
		left,
		right,
	}
}

func NewJSONSetCompleteness(node *SetCompletenessProofNode, leftChild []byte, rightChild []byte, isLeft bool) JSONSetCompletenessProofNode {
	return JSONSetCompletenessProofNode{
		node.Hash,
		leftChild,
		rightChild,
		node.PartialPrefix,
		node.CumulativePrefix,
		isLeft,
	}
}
