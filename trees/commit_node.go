package trees

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/tap-group/tdsvc/crypto/p256"
	"github.com/tap-group/tdsvc/util"
)

type commitInternalNode struct {
	parent      commitNode
	leftChild   commitNode
	rightChild  commitNode
	numLeaves   int
	commitments [][]byte
	childHash   []byte
	hash        [32]byte
}

type commitLeafNode struct {
	parent      commitNode
	uid         int
	time        int
	seeds       []int
	values      []int
	commitments [][]byte
	idHash      [32]byte
	hash        [32]byte
}

type commitNode interface {
	isLeafNode() bool
	isLeftChild() bool
	getCommitment(int) []byte
	getCommitments() [][]byte
	getValue(int) int
	getValues() []int
	getHash() [32]byte
	getChildHash() []byte
	setParent(parent commitNode)
	getParent() commitNode
	getRightChild() commitNode
	getLeftChild() commitNode
	getNumLeaves() int
	getSibling() commitNode
	setLeftChild(child commitNode)
	setRightChild(child commitNode)
	updateInternalData()
	copy() commitNode
	toLeafNode() *commitLeafNode
	toInternalNode() *commitInternalNode
	// serialize() ([]byte, error)
}

func getCommitmentAsBytes(value int, seed int) []byte {
	H, err0 := p256.MapToGroup(COMMIT_SEEDH)
	util.Check(err0)
	C, err1 := p256.CommitG1(big.NewInt(int64(value)), big.NewInt(int64(seed)), H)
	util.Check(err1)
	bytes, err2 := json.Marshal(C)
	util.Check(err2)
	return bytes
}

func newCommitLeaf(values, seeds []int, uid, time int) *commitLeafNode {
	node := &commitLeafNode{
		values: values,
		seeds:  seeds,
		uid:    uid,
		time:   time,
	}
	node.updateInternalData()
	return node
}

func newCommitLeafFromCommitments(commitments []*p256.P256, idHash [32]byte) *commitLeafNode {
	commitmentBytes := make([][]byte, len(commitments))
	for i := range commitmentBytes {
		newCommitmentBytes, err2 := json.Marshal(commitments[i])
		commitmentBytes[i] = newCommitmentBytes
		util.Check(err2)
	}
	node := &commitLeafNode{
		commitments: commitmentBytes,
		idHash:      idHash,
	}
	node.updateInternalData()
	return node
}

func getCommitSibling(node commitNode) commitNode {
	if node.getParent() == nil {
		return nil
	}
	if node.isLeftChild() {
		return node.getParent().getRightChild()
	}
	return node.getParent().getLeftChild()
}

func isLeftChild(node commitNode) bool {
	if node.getParent() == nil {
		return false
	}
	return node.getParent().getLeftChild() == node
}

func (node *commitLeafNode) isLeafNode() bool               { return true }
func (node *commitLeafNode) isLeftChild() bool              { return isLeftChild(node) }
func (node *commitLeafNode) getCommitment(k int) []byte     { return node.commitments[k] }
func (node *commitLeafNode) getCommitments() [][]byte       { return node.commitments }
func (node *commitLeafNode) getValue(k int) int             { return node.values[k] }
func (node *commitLeafNode) getValues() []int               { return node.values }
func (node *commitLeafNode) getHash() [32]byte              { return node.hash }
func (node *commitLeafNode) getChildHash() []byte           { return nil }
func (node *commitLeafNode) setParent(parent commitNode)    { node.parent = parent }
func (node *commitLeafNode) getParent() commitNode          { return node.parent }
func (node *commitLeafNode) getRightChild() commitNode      { return nil }
func (node *commitLeafNode) getLeftChild() commitNode       { return nil }
func (node *commitLeafNode) getNumLeaves() int              { return 1 }
func (node *commitLeafNode) getSibling() commitNode         { return getCommitSibling(node) }
func (node *commitLeafNode) setLeftChild(child commitNode)  {}
func (node *commitLeafNode) setRightChild(child commitNode) {}
func (node *commitLeafNode) updateInternalData() {
	if len(node.values) > 0 {
		node.commitments = make([][]byte, len(node.values))
		for i := range node.values {
			node.commitments[i] = getCommitmentAsBytes(node.values[i], node.seeds[i])
		}
		node.idHash = util.IdHash(node.uid, node.time)
	}

	commitmentHash := util.HashCommitmentsAndCount(node.commitments, 1)
	node.hash = util.Hash32(commitmentHash[:], node.idHash[:])
}
func (node *commitLeafNode) copy() commitNode {
	// deep copy byte arrays
	copyCommitments := make([][]byte, len(node.commitments))
	for i := range copyCommitments {
		copyCommitments[i] = make([]byte, len(node.commitments[i]))
		copy(copyCommitments[i], node.commitments[i])
	}
	var copyHash [32]byte
	for i := 0; i < 32; i++ {
		copyHash[i] = node.hash[i]
	}
	var copyIdHash [32]byte
	for i := 0; i < 32; i++ {
		copyIdHash[i] = node.idHash[i]
	}
	result := &commitLeafNode{
		commitments: copyCommitments,
		hash:        copyHash,
		idHash:      copyIdHash,
	}
	return result
}
func (node *commitLeafNode) toLeafNode() *commitLeafNode {
	return &commitLeafNode{
		parent:      node.parent,
		uid:         node.uid,
		time:        node.time,
		values:      node.values,
		seeds:       node.seeds,
		commitments: node.commitments,
		idHash:      node.idHash,
		hash:        node.hash,
	}
}
func (node *commitLeafNode) toInternalNode() *commitInternalNode {
	fmt.Println("warning: cannot cast from leaf node to internal node")
	return &commitInternalNode{}
}

// helper function of internal nodes
func GetInternalNodeParams(leftNode commitNode, rightNode commitNode) (int, [][]byte, []byte, [32]byte) {
	if leftNode == nil || leftNode.getNumLeaves() == 0 {
		if rightNode == nil {
			return -1, nil, nil, *new([32]byte)
		}
		childHash := rightNode.getHash()
		newNumLeaves := rightNode.getNumLeaves()
		newCommitmentBytes := rightNode.getCommitments()
		hashComp := util.HashCommitmentsAndCount(newCommitmentBytes, newNumLeaves)
		newHash := util.Hash32(append(childHash[:], hashComp[:]...))
		return newNumLeaves, newCommitmentBytes, childHash[:], newHash
	}
	if rightNode == nil || rightNode.getNumLeaves() == 0 {
		childHash := leftNode.getHash()
		newNumLeaves := leftNode.getNumLeaves()
		newCommitmentBytes := leftNode.getCommitments()
		hashComp := util.HashCommitmentsAndCount(newCommitmentBytes, newNumLeaves)
		newHash := util.Hash32(append(childHash[:], hashComp[:]...))
		return newNumLeaves, newCommitmentBytes, childHash[:], newHash
	}
	leftHash := leftNode.getHash()
	rightHash := rightNode.getHash()
	childHash := append(leftHash[:], rightHash[:]...)
	n := len(leftNode.getCommitments())
	newCommitmentBytes := make([][]byte, n)
	for i := 0; i < n; i++ {
		var leftCommitment, rightCommitment *p256.P256
		_ = json.Unmarshal(leftNode.getCommitment(i), &leftCommitment)
		_ = json.Unmarshal(rightNode.getCommitment(i), &rightCommitment)

		newCommitment := new(p256.P256).Add(leftCommitment, rightCommitment)
		newCommitmentBytesTemp, err2 := json.Marshal(newCommitment)
		newCommitmentBytes[i] = newCommitmentBytesTemp
		util.Check(err2)
	}
	newNumLeaves := leftNode.getNumLeaves() + rightNode.getNumLeaves()
	hashComp := util.HashCommitmentsAndCount(newCommitmentBytes, newNumLeaves)
	newHash := util.Hash32(append(childHash, hashComp[:]...))

	return newNumLeaves, newCommitmentBytes, childHash, newHash
}

func (node *commitInternalNode) isLeafNode() bool               { return false }
func (node *commitInternalNode) isLeftChild() bool              { return isLeftChild(node) }
func (node *commitInternalNode) getCommitment(k int) []byte     { return node.commitments[k] }
func (node *commitInternalNode) getCommitments() [][]byte       { return node.commitments }
func (node *commitInternalNode) getValue(k int) int             { return -1 }
func (node *commitInternalNode) getValues() []int               { return nil }
func (node *commitInternalNode) getHash() [32]byte              { return node.hash }
func (node *commitInternalNode) getChildHash() []byte           { return node.childHash }
func (node *commitInternalNode) setParent(parent commitNode)    { node.parent = parent }
func (node *commitInternalNode) getParent() commitNode          { return node.parent }
func (node *commitInternalNode) getRightChild() commitNode      { return node.rightChild }
func (node *commitInternalNode) getLeftChild() commitNode       { return node.leftChild }
func (node *commitInternalNode) getNumLeaves() int              { return node.numLeaves }
func (node *commitInternalNode) getSibling() commitNode         { return getCommitSibling(node) }
func (node *commitInternalNode) setLeftChild(child commitNode)  { node.leftChild = child }
func (node *commitInternalNode) setRightChild(child commitNode) { node.rightChild = child }
func (node *commitInternalNode) updateInternalData() {
	node.numLeaves, node.commitments, node.childHash, node.hash = GetInternalNodeParams(node.leftChild, node.rightChild)
}
func (node *commitInternalNode) copy() commitNode {
	// deep copy byte arrays
	copyCommitments := make([][]byte, len(node.commitments))
	for i := range copyCommitments {
		copyCommitments[i] = make([]byte, len(node.commitments[i]))
		copy(copyCommitments[i], node.commitments[i])
	}
	var copyHash [32]byte
	for i := 0; i < 32; i++ {
		copyHash[i] = node.hash[i]
	}
	copyChildHash := make([]byte, len(node.childHash))
	copy(copyChildHash, node.childHash)
	result := &commitInternalNode{
		commitments: copyCommitments,
		hash:        copyHash,
		childHash:   copyChildHash,
		numLeaves:   node.getNumLeaves(),
	}
	return result
}
func (node *commitInternalNode) toLeafNode() *commitLeafNode {
	fmt.Println("warning: cannot cast from internal node to leaf node")
	return &commitLeafNode{}
}
func (node *commitInternalNode) toInternalNode() *commitInternalNode {
	return &commitInternalNode{
		parent:      node.parent,
		commitments: node.commitments,
		hash:        node.hash,
		childHash:   node.childHash,
		leftChild:   node.leftChild,
		rightChild:  node.rightChild,
		numLeaves:   node.numLeaves,
	}
}
