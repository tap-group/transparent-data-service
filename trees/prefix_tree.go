package trees

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/tap-group/tdsvc/util"
)

type PrefixTree struct {
	root       *prefixInternalNode
	isComplete bool
}

// for node on path to root, store onpath partial prefix, and the hash of the offpath child,
type forNodeOnCopath struct {
	// for root there is no partial prefix
	PartialPrefix []byte
	//the child node that isn't on path (struct for starting node stores node itself)
	OtherChildHash []byte
	IsLeft         bool
}

// MembershipProof ...
type MembershipProof struct {
	LeafPartialPrefix []byte
	CopathNodes       []forNodeOnCopath // first is leaf's sibling, last is root
}

// NonMembershipProof ...
type NonMembershipProof struct {
	EndNodeHash          []byte            // for empty nodes, will be nil
	EndNodePartialPrefix []byte            // for empty nodes, will be the (one) next-expected byte
	CopathNodes          []forNodeOnCopath //first is node at bottom of path, last is root
}

type SetCompletenessProofNode struct {
	CumulativePrefix []byte
	PartialPrefix    []byte
	IsLeft           bool
	Parent           *SetCompletenessProofNode
	LeftChild        *SetCompletenessProofNode
	RightChild       *SetCompletenessProofNode
	Hash             []byte
}

func (s *SetCompletenessProofNode) SetCumulativePrefix(p []byte) {
	s.CumulativePrefix = p
}

func (s *SetCompletenessProofNode) SetParent(n *SetCompletenessProofNode) {
	s.Parent = n
}

func (s *SetCompletenessProofNode) SetLeftChild(n *SetCompletenessProofNode) {
	s.LeftChild = n
}

func (s *SetCompletenessProofNode) SetRightChild(n *SetCompletenessProofNode) {
	s.RightChild = n
}

func (s *SetCompletenessProofNode) GetParent() *SetCompletenessProofNode {
	return s.Parent
}

func (s *SetCompletenessProofNode) GetLeftChild() *SetCompletenessProofNode {
	return s.LeftChild
}

func (s *SetCompletenessProofNode) GetRightChild() *SetCompletenessProofNode {
	return s.RightChild
}

func (s *SetCompletenessProofNode) GetPartialPrefix() []byte {
	return s.PartialPrefix
}

func (s *SetCompletenessProofNode) GetCumulativePrefix() []byte {
	return s.CumulativePrefix
}

func (s *SetCompletenessProofNode) GetHash() []byte {
	return s.Hash
}

func NewPrefixTree() *PrefixTree {

	res := &PrefixTree{
		root: &prefixInternalNode{
			parent:        nil,
			hash:          nil,
			leftChild:     nil,
			rightChild:    nil,
			partialPrefix: nil,
		},
		isComplete: false,
	}

	return res
}

func (tree *PrefixTree) GetNodeCount() int {
	return tree.root.getNumNodes()
}

func (tree *PrefixTree) GetSize() int {
	return tree.root.getSize()
}

func (tree *PrefixTree) GetRoot() *prefixInternalNode {
	return tree.root
}

func (tree *PrefixTree) PrefixAppend(bitPrefix []byte, value []byte) (err error) {
	var prev prefixNode
	var curr prefixNode = tree.root
	i := uint32(0)

	for i < uint32(len(bitPrefix)) {

		prev = curr
		curr = curr.getChild(bitPrefix, i)

		if curr == nil { //this child had not been made yet
			leaf := newPrefixLeafNode(prev, value, bitPrefix[i:])
			tree.updateHashesFromLeaf(leaf)
			return
		}

		j := uint32(0)
		for j < uint32(len(curr.getPartialPrefix())) {
			if bitPrefix[i] == curr.getPartialPrefix()[j] {
				i++
				j++
			} else {
				newParent := splitCompressedNode(curr, prev, j)
				curr.updateHash()
				leaf := newPrefixLeafNode(newParent, value, bitPrefix[i:])
				tree.updateHashesFromLeaf(leaf)
				return
			}
		}
	}
	leaf := curr
	leaf.setValue(value)
	tree.updateHashesFromLeaf(leaf)
	return
}

func (tree *PrefixTree) getLeaf(bitPrefix []byte) prefixNode {

	var curr prefixNode = tree.root
	i := uint32(0)

	for i < uint32(len(bitPrefix)) {
		curr = curr.getChild(bitPrefix, i)
		if curr == nil {
			return nil //key doesn't exist in tree
		}
		partialPrefix := curr.getPartialPrefix()
		if bytes.Equal(bitPrefix[i:i+uint32(len(partialPrefix))], partialPrefix) {
			i += uint32(len(partialPrefix))
			continue
		} else {
			return nil //key doesn't exist in tree
		}
	}
	return curr
}

func DoesPrefixOverlapRange(valRange [][]uint32, prefix []byte, fieldSize int) bool {
	numBitsPerField := fieldSize * 8
	fieldMin := 0
	fieldMax := 0
	for i := 0; i < numBitsPerField; i++ {
		m := util.PowInt(2, numBitsPerField-i-1)
		if i < len(prefix) {
			if prefix[i] == 1 {
				fieldMin += m
				fieldMax += m
			}
		} else {
			fieldMax += m
		}
	}
	// fmt.Println("[" + strconv.Itoa(fieldMin) + ", " + strconv.Itoa(fieldMax) + "] vs [" + strconv.Itoa(valRange[0][0]) + ", " + strconv.Itoa(valRange[0][1]) + ")")
	if int(valRange[0][1]) <= fieldMin || int(valRange[0][0]) > fieldMax {
		return false
	}
	if len(prefix) > numBitsPerField {
		return DoesPrefixOverlapRange(valRange[1:], prefix[numBitsPerField:], fieldSize)
	}
	return valRange[0][1]-valRange[0][0] > 0
}

// func CheckSetCompletenessProof(valRange [][]int, proofs []*MembershipProof, leafValues [][]byte, fieldSize int) []forNodeOnCopath {
// 	validMap := make(map[[32]byte]bool)

// 	// check which nodes are part of one of the paths
// 	for i, proof := range proofs {
// 		if proof != nil {
// 			h := util.Hash(proof.LeafPartialPrefix, leafValues[i])
// 			nodeHash := util.Hash32(h)
// 			validMap[nodeHash] = true
// 			for i := 0; i < len(proof.CopathNodes); i++ {
// 				node := proof.CopathNodes[i]
// 				if !node.IsLeft {
// 					h = util.Hash(node.PartialPrefix, node.OtherChildHash, h)
// 				} else {
// 					h = util.Hash(node.PartialPrefix, h, node.OtherChildHash)
// 				}
// 				nodeHash := util.Hash32(h)
// 				validMap[nodeHash] = true
// 			}
// 		}
// 	}

// 	invalidNodes := make([]forNodeOnCopath, 0)
// 	// the remaining copath nodes must be ...
// 	for _, proof := range proofs {
// 		fmt.Println("---")
// 		cumulativePartialPrefix := make([]byte, 0)
// 		if proof != nil {
// 			for i := len(proof.CopathNodes) - 1; i >= 0; i-- {
// 				if proof.CopathNodes[i].OtherChildHash != nil {
// 					cumulativePartialPrefix = append(cumulativePartialPrefix, proof.CopathNodes[i].PartialPrefix...)
// 					partOfPath := validMap[util.Hash32(proof.CopathNodes[i].OtherChildHash)]
// 					var cumulativePartialPrefixCo []byte
// 					if proof.CopathNodes[i].IsLeft {
// 						cumulativePartialPrefixCo = append(cumulativePartialPrefix, 1)
// 					} else {
// 						cumulativePartialPrefixCo = append(cumulativePartialPrefix, 0)
// 					}
// 					prefixOverlap := DoesPrefixOverlapRange(valRange, cumulativePartialPrefixCo, fieldSize)
// 					if !partOfPath && prefixOverlap {
// 						invalidNodes = append(invalidNodes, proof.CopathNodes[i])
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return invalidNodes
// }

// func AddCompletenessProofs(node forNodeOnCopath, completenessProofs []*MembershipProof) {

// }

func ExtendSetCompletenessProof(node prefixNode, proofNode *SetCompletenessProofNode, cumulativePrefix []byte, valRange [][]uint32, fieldSize int) {
	cPrefixCopy := append([]byte{}, cumulativePrefix...)
	newCPrefix := append(cPrefixCopy, node.getPartialPrefix()...)
	proofNode.SetCumulativePrefix(newCPrefix)
	if DoesPrefixOverlapRange(valRange, newCPrefix, fieldSize) {
		leftChild := node.getLeftChild()
		rightChild := node.getRightChild()
		if leftChild != nil {
			// fmt.Print("left: ")
			// fmt.Println(newCPrefix)
			leftNode := &SetCompletenessProofNode{
				PartialPrefix: leftChild.getPartialPrefix(),
				Hash:          leftChild.getHash(),
				IsLeft:        true,
			}
			proofNode.SetLeftChild(leftNode)
			leftNode.SetParent(proofNode)
			ExtendSetCompletenessProof(leftChild, leftNode, newCPrefix, valRange, fieldSize)
		}
		if rightChild != nil {
			// fmt.Print("right: ")
			// fmt.Println(newCPrefix)
			rightNode := &SetCompletenessProofNode{
				PartialPrefix: rightChild.getPartialPrefix(),
				Hash:          rightChild.getHash(),
				IsLeft:        false,
			}
			proofNode.SetRightChild(rightNode)
			rightNode.SetParent(proofNode)
			ExtendSetCompletenessProof(rightChild, rightNode, newCPrefix, valRange, fieldSize)
		}
	}
}

func (node *SetCompletenessProofNode) CheckHash() bool {
	if node.GetLeftChild() != nil && node.GetRightChild() != nil {
		h := util.Hash(node.GetPartialPrefix(), node.GetLeftChild().GetHash(), node.GetRightChild().GetHash())
		if !reflect.DeepEqual(h, node.GetHash()) {
			return false
		}
		return node.GetLeftChild().CheckHash() && node.GetRightChild().CheckHash()
	} else if node.GetLeftChild() != nil {
		h := util.Hash(node.GetPartialPrefix(), node.GetLeftChild().GetHash(), nil)
		if !reflect.DeepEqual(h, node.GetHash()) {
			return false
		}
		return node.GetLeftChild().CheckHash()
	} else if node.GetRightChild() != nil {
		h := util.Hash(node.GetPartialPrefix(), nil, node.GetLeftChild().GetHash())
		if !reflect.DeepEqual(h, node.GetHash()) {
			return false
		}
		return node.GetRightChild().CheckHash()
	}
	return true
}

func FindValidNodes(node *SetCompletenessProofNode, validNodes []*SetCompletenessProofNode, cumulativePrefix []byte, valRange [][]uint32, fieldSize int) []*SetCompletenessProofNode {
	cPrefixCopy := append([]byte{}, cumulativePrefix...)
	newCPrefix := append(cPrefixCopy, node.GetPartialPrefix()...)
	if node.GetLeftChild() == nil && node.GetRightChild() == nil {
		if DoesPrefixOverlapRange(valRange, newCPrefix, fieldSize) {
			return append(validNodes, node)
		}
		return validNodes
	}
	if node.GetLeftChild() != nil && node.GetRightChild() != nil {
		newValidNodes := FindValidNodes(node.GetLeftChild(), validNodes, newCPrefix, valRange, fieldSize)
		return FindValidNodes(node.GetRightChild(), newValidNodes, newCPrefix, valRange, fieldSize)
	}
	if node.GetLeftChild() != nil {
		return FindValidNodes(node.GetLeftChild(), validNodes, newCPrefix, valRange, fieldSize)
	}
	if node.GetRightChild() != nil {
		return FindValidNodes(node.GetRightChild(), validNodes, newCPrefix, valRange, fieldSize)
	}
	fmt.Println("warning: dummy return statement reached")
	return nil
}

// func (node *SetCompletenessProofNode) VerifyPrefixes(valRange [][]int, prefixes [][]byte, fieldSize int) bool {
// 	validNodes := make([]*SetCompletenessProofNode, 0)
// 	validNodes = FindValidNodes(node, validNodes, make([]byte, 0), valRange, fieldSize)

// 	fmt.Println(strconv.Itoa(len(validNodes)) + " valid prefix nodes in set completeness proof")

// 	for _, node := range validNodes {
// 		fmt.Println(node.GetHash())
// 		for _, prefix := range prefixes {
// 			fmt.Println(prefix)
// 		}
// 	}

// 	fmt.Println(len(validNodes))

// 	return false
// }

func (tree *PrefixTree) GenerateSetCompletenessProof(valRange [][]uint32, fieldSize int) *SetCompletenessProofNode {
	proofRoot := &SetCompletenessProofNode{
		PartialPrefix: tree.root.getPartialPrefix(),
		Hash:          tree.root.getHash(),
	}
	ExtendSetCompletenessProof(tree.root, proofRoot, make([]byte, 0), valRange, fieldSize)
	return proofRoot
}

// func (tree *PrefixTree) GenerateSetCompletenessProofOld(valRange [][]int, prefixes [][]byte, fieldSize int) (membershipProofs []*MembershipProof, leafValues [][]byte) {
// 	leaves := make([]prefixNode, len(prefixes))
// 	membershipProofs = make([]*MembershipProof, len(prefixes))
// 	leafValues = make([][]byte, len(prefixes))

// 	numBits := -1

// 	// first generate the inclusion proofs
// 	for i, prefix := range prefixes {
// 		bitPrefix := util.ConvertBytesToBits(prefix)
// 		if numBits > -1 && numBits != len(bitPrefix) {
// 			fmt.Println("warning: not all prefixes have been converted to bit prefixes of same length")
// 		}
// 		numBits = len(bitPrefix)
// 		leaves[i] = tree.getLeaf(bitPrefix)
// 		membershipProofs[i], leafValues[i] = tree.GenerateMembershipProof(bitPrefix)
// 	}

// 	// check all copath leaves to see if they meet the completeness conditions - if not, add proofs
// 	invalidNodes := CheckSetCompletenessProof(valRange, membershipProofs, leafValues, fieldSize)
// 	if len(invalidNodes) > 0 {
// 		completenessProofs := make([]*MembershipProof, 0)
// 		for _, node := range invalidNodes {
// 			AddCompletenessProofs(node, completenessProofs)
// 		}
// 		fmt.Println("invalid nodes! " + strconv.Itoa(len(invalidNodes)))
// 	}

// 	return membershipProofs, leafValues
// }

func (tree *PrefixTree) GenerateMembershipProof(bitPrefix []byte) (proof *MembershipProof, leafValue []byte) {
	leaf := tree.getLeaf(bitPrefix)
	if leaf == nil {
		return nil, nil
	}

	if !leaf.isLeafNode() {
		panic("prefix path should end with a leaf, but does not")
	}

	copath := tree.buildCopathFromNode(leaf)
	return &MembershipProof{
		LeafPartialPrefix: leaf.getPartialPrefix(),
		CopathNodes:       copath,
	}, leaf.getValue()
}

func (tree *PrefixTree) GenerateNonMembershipProof(prefix []byte) *NonMembershipProof {

	var prev prefixNode
	var curr prefixNode = tree.root
	i := uint32(0)

	for i < uint32(len(prefix)) {

		prev = curr
		curr = curr.getChild(prefix, i)

		if curr == nil {
			if prev != tree.root {
				panic("root should be the only internal node that can have <2 children")
			}
			missingNode := &prefixInternalNode{
				parent:        tree.root,
				partialPrefix: []byte{prefix[i]},
			}
			return &NonMembershipProof{
				EndNodeHash:          nil,
				EndNodePartialPrefix: missingNode.getPartialPrefix(),
				CopathNodes:          tree.buildCopathFromNode(missingNode),
			}
		}
		partialPrefix := curr.getPartialPrefix()
		if bytes.Equal(prefix[i:i+uint32(len(partialPrefix))], partialPrefix) {
			i += uint32(len(partialPrefix))
			continue
		} else {
			return &NonMembershipProof{
				EndNodeHash:          curr.getHash(),
				EndNodePartialPrefix: curr.getPartialPrefix(),
				CopathNodes:          tree.buildCopathFromNode(curr),
			}
		}
	}
	return nil //key exists
}

func (proof *MembershipProof) RebuildRootHash(leafValue []byte) []byte {
	h := util.Hash(nil)
	if proof != nil {
		h = util.Hash(proof.LeafPartialPrefix, leafValue)
		for i := 0; i < len(proof.CopathNodes); i++ {
			node := proof.CopathNodes[i]
			if !node.IsLeft {
				h = util.Hash(node.PartialPrefix, node.OtherChildHash, h)
			} else {
				h = util.Hash(node.PartialPrefix, h, node.OtherChildHash)
			}
		}
	}
	return h
}

func (proof *NonMembershipProof) RebuildRootHash() []byte {
	h := util.Hash(nil)
	if proof != nil {
		h = proof.EndNodeHash
		for i := 0; i < len(proof.CopathNodes); i++ {
			node := proof.CopathNodes[i]
			if !node.IsLeft {
				h = util.Hash(node.PartialPrefix, node.OtherChildHash, h)
			} else {
				h = util.Hash(node.PartialPrefix, h, node.OtherChildHash)
			}
		}
	}
	return h
}

func (tree *PrefixTree) buildCopathFromNode(startingNode prefixNode) []forNodeOnCopath {
	copath := []forNodeOnCopath{}
	curr := startingNode
	for curr.getParent() != nil {
		var siblingHash []byte
		if curr.getSibling() != nil {
			siblingHash = curr.getSibling().getHash()
		}
		isLeft := true
		if curr == curr.getParent().getRightChild() {
			isLeft = false
		}
		copath = append(copath,
			forNodeOnCopath{
				IsLeft:         isLeft,
				PartialPrefix:  curr.getParent().getPartialPrefix(),
				OtherChildHash: siblingHash,
			})
		curr = curr.getParent()
	}
	if curr != tree.root {
		panic("copath should end at root, there is a node on path missing a parent value")
	}

	return copath
}

func splitCompressedNode(nodeToSplit prefixNode, parent prefixNode, index uint32) *prefixInternalNode {
	prefixLength := uint32(len(nodeToSplit.getPartialPrefix()))
	if prefixLength <= 1 {
		panic("can't split a non-compressed node")
	} else if index == 0 || index >= prefixLength {
		panic("given index doesn't split the prefix into 2 peices")
	}
	intermediateNode := newPrefixInteriorNode(parent, nodeToSplit.getPartialPrefix()[0:index])

	nodeToSplit.setPartialPrefix(nodeToSplit.getPartialPrefix()[index:])
	intermediateNode.addChild(nodeToSplit)

	return intermediateNode
}

func (tree *PrefixTree) updateHashesFromLeaf(leaf prefixNode) {

	if !leaf.isLeafNode() {
		panic("updateHashesFromLeaf was passed internalNode as argument")
	}

	curr := leaf
	for curr != tree.root {
		curr.updateHash()
		curr = curr.getParent()
	}
	tree.root.updateHash()
}

func (tree *PrefixTree) GetHash() []byte {
	return tree.root.hash
}
