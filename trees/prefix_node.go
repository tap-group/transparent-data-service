package trees

import (
	"encoding/binary"

	"github.com/tap-group/tdsvc/util"
)

// Internal node in Prefix Tree
type prefixInternalNode struct {
	parent        prefixNode
	hash          []byte
	leftChild     prefixNode
	rightChild    prefixNode
	partialPrefix []byte
}

// Leaf node representation in Prefix Tree
type prefixLeafNode struct {
	parent        prefixNode
	hash          []byte
	value         []byte
	partialPrefix []byte
}

func newPrefixInteriorNode(parent prefixNode, partialPrefix []byte) *prefixInternalNode {
	res := &prefixInternalNode{
		hash:          nil,
		leftChild:     nil,
		rightChild:    nil,
		partialPrefix: partialPrefix,
	}
	parent.addChild(res)
	return res
}

func newPrefixLeafNode(parent prefixNode, value []byte, partialPrefix []byte) *prefixLeafNode {
	if partialPrefix == nil || len(partialPrefix) <= 0 {
		panic("cannot create a leaf branch without a partial prefix")
	}

	valueCopy := append([]byte{}, value...)

	res := &prefixLeafNode{
		hash:          nil,
		value:         valueCopy,
		partialPrefix: partialPrefix,
	}
	parent.addChild(res)
	return res
}

// helper function for internalNode and leafNode funcs
func getPrefixSibling(node prefixNode) prefixNode {
	if node.getParent() == nil {
		return nil
	}
	if node.isLeftChild() {
		return node.getParent().getRightChild()
	}
	return node.getParent().getLeftChild()
}

type prefixNode interface {
	isLeafNode() bool
	isLeftChild() bool
	getHash() []byte
	setParent(parent prefixNode)
	getParent() prefixNode
	getRightChild() prefixNode
	getLeftChild() prefixNode
	getPartialPrefix() []byte
	setPartialPrefix(newPrefix []byte)
	getValue() []byte
	setValue(value []byte)
	getSibling() prefixNode
	addChild(child prefixNode)
	getChild(prefix []byte, nextPrefixByteIndex uint32) prefixNode
	updateHash()
	serialize() ([]byte, error)
	getNumNodes() int
	getSize() int
}

func (node *prefixInternalNode) isLeafNode() bool                  { return false }
func (node *prefixInternalNode) isLeftChild() bool                 { return node.partialPrefix[0] == 0 }
func (node *prefixInternalNode) getHash() []byte                   { return node.hash }
func (node *prefixInternalNode) setParent(parent prefixNode)       { node.parent = parent }
func (node *prefixInternalNode) getParent() prefixNode             { return node.parent }
func (node *prefixInternalNode) getRightChild() prefixNode         { return node.rightChild }
func (node *prefixInternalNode) getLeftChild() prefixNode          { return node.leftChild }
func (node *prefixInternalNode) getPartialPrefix() []byte          { return node.partialPrefix }
func (node *prefixInternalNode) setPartialPrefix(newPrefix []byte) { node.partialPrefix = newPrefix }
func (node *prefixInternalNode) getValue() []byte                  { return nil }
func (node *prefixInternalNode) setValue(value []byte)             {}
func (node *prefixInternalNode) getSibling() prefixNode            { return getPrefixSibling(node) }
func (node *prefixInternalNode) addChild(child prefixNode) {
	child.setParent(node)
	if child.getPartialPrefix()[0] == 0 {
		node.leftChild = child
	} else {
		node.rightChild = child
	}
}
func (node *prefixInternalNode) getChild(prefix []byte, nextPrefixByteIndex uint32) prefixNode {
	nextPrefixByte := prefix[nextPrefixByteIndex]
	if nextPrefixByte == 0 {
		return node.getLeftChild()
	}
	return node.getRightChild()

}

func (node *prefixInternalNode) updateHash() {
	var leftHash, rightHash []byte
	if node.leftChild != nil {
		leftHash = node.leftChild.getHash()
	}
	if node.rightChild != nil {
		rightHash = node.rightChild.getHash()
	}
	node.hash = util.Hash(node.partialPrefix, leftHash, rightHash)
}

func (node *prefixInternalNode) getNumNodes() int {
	total := 1
	// right tree, if exists
	if node.getRightChild() != nil {
		total += node.getRightChild().getNumNodes()
	}

	// left tree, if exists
	if node.getLeftChild() != nil {
		total += node.getLeftChild().getNumNodes()
	}

	return total
}

func (node *prefixInternalNode) getSize() int {

	// pointer to parent, left and right child
	total := 64 * 3

	// size of partialPrefix and hash
	total += binary.Size(node.getHash())
	if node.getPartialPrefix() != nil {
		// total += binary.Size(node.getPartialPrefix()) / 8
		total += binary.Size(node.getPartialPrefix())
	}
	// fmt.Println(total)

	// right tree, if exists
	if node.getRightChild() != nil {
		total += node.getRightChild().getSize()
	}

	// left tree, if exists
	if node.getLeftChild() != nil {
		total += node.getLeftChild().getSize()
	}

	return total
}

func (node *prefixLeafNode) isLeafNode() bool                  { return true }
func (node *prefixLeafNode) isLeftChild() bool                 { return node.partialPrefix[0] == 0 }
func (node *prefixLeafNode) getHash() []byte                   { return node.hash }
func (node *prefixLeafNode) setParent(parent prefixNode)       { node.parent = parent }
func (node *prefixLeafNode) getParent() prefixNode             { return node.parent }
func (node *prefixLeafNode) getRightChild() prefixNode         { return nil }
func (node *prefixLeafNode) getLeftChild() prefixNode          { return nil }
func (node *prefixLeafNode) getPartialPrefix() []byte          { return node.partialPrefix }
func (node *prefixLeafNode) setPartialPrefix(newPrefix []byte) { node.partialPrefix = newPrefix }
func (node *prefixLeafNode) getValue() []byte                  { return node.value }
func (node *prefixLeafNode) setValue(value []byte) {
	node.value = value
}
func (node *prefixLeafNode) getSibling() prefixNode    { return getPrefixSibling(node) }
func (node *prefixLeafNode) addChild(child prefixNode) {}
func (node *prefixLeafNode) getChild(prefix []byte, nextPrefixByteIndex uint32) prefixNode {
	return nil
}
func (node *prefixLeafNode) updateHash() { node.hash = util.Hash(node.partialPrefix, node.value) }

func (node *prefixLeafNode) getNumNodes() int {
	return 1
}

func (node *prefixLeafNode) getSize() int {

	// pointer to parent
	total := 64

	// size of partialPrefix and hash
	total += binary.Size(node.getPartialPrefix()) + binary.Size(node.getHash())

	// size of KeyHash values
	total += binary.Size(node.getValue())

	return total
}
