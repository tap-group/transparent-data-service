package trees

import (
	"fmt"
	"log"
	"reflect"

	"github.com/tap-group/tdsvc/crypto/p256"
)

var COMMIT_SEEDH = "BulletproofsDoesNotNeedTrustedSetupH"

type CommitTree struct {
	root *commitInternalNode
}

type CommitMembershipProof struct {
	LeafCommitments     []*forNodeCommitment
	LeafIdHash          [32]byte
	LeafHash            [32]byte
	Copath              []*forNodeOnCommitCopath
	CopathIsLeftSibling []bool
}

type forNodeOnCommitCopath struct {
	NumLeaves   int
	Commitments []*forNodeCommitment
	ChildHash   []byte
	Hash        [32]byte
}

type forNodeCommitment struct {
	Commitment []byte
}

func parseFromForNodeCommitments(a []*forNodeCommitment) [][]byte {
	result := make([][]byte, len(a))
	for i := range a {
		result[i] = a[i].Commitment
	}
	return result
}

func parseToForNodeCommitments(a [][]byte) []*forNodeCommitment {
	result := make([]*forNodeCommitment, len(a))
	for i := range a {
		result[i] = &forNodeCommitment{Commitment: a[i]}
	}
	return result
}

func buildUpTreePerLayer(ns []commitNode) commitNode {

	if len(ns) <= 0 {
		return nil
	}
	if len(ns) == 1 {
		return ns[0]
	}
	var nodes []commitNode
	for i := 0; i < len(ns); i += 2 {
		var leftNode, rightNode commitNode
		if i+1 == len(ns) {
			nodes = append(nodes, ns[i])
		} else {
			leftNode = ns[i]
			rightNode = ns[i+1]
			n := &commitInternalNode{
				leftChild:  leftNode,
				rightChild: rightNode,
			}
			n.updateInternalData()
			nodes = append(nodes, n)
			leftNode.setParent(n)
			rightNode.setParent(n)
		}
	}
	return buildUpTreePerLayer(nodes)
}

func (p *CommitMembershipProof) GetSize() int {
	return len(p.CopathIsLeftSibling)
}

func (p *CommitMembershipProof) IsLeftSibling(i int) bool {
	return p.CopathIsLeftSibling[i]
}

func (p *CommitMembershipProof) GetNumLeftNodes() int {
	result := 0
	n := len(p.CopathIsLeftSibling)
	for i := 0; i < n-1; i++ {
		if p.CopathIsLeftSibling[i] {
			result += p.Copath[i].NumLeaves
		}
	}
	return result
}

func (p *CommitMembershipProof) GetNumRightNodes() int {
	result := 0
	n := len(p.CopathIsLeftSibling)
	for i := 0; i < n-1; i++ {
		if !p.CopathIsLeftSibling[i] {
			result += p.Copath[i].NumLeaves
		}
	}
	return result
}

func (p *CommitMembershipProof) GetTotalNumNodes() int {
	result := 0
	n := len(p.CopathIsLeftSibling)
	for i := 0; i < n-1; i++ {
		result += p.Copath[i].NumLeaves
	}
	return result + 1
}

func (p *CommitMembershipProof) GetLeafCommitments() [][]byte {
	return parseFromForNodeCommitments(p.LeafCommitments)
}

func (p *CommitMembershipProof) GetLeafCommitment(k int) []byte {
	return parseFromForNodeCommitments(p.LeafCommitments)[k]
}

func (tree *CommitTree) GetRootHash() [32]byte {
	return tree.root.getHash()
}

func (tree *CommitTree) GetNumLeaves() int {
	return tree.root.getNumLeaves()
}

func (tree *CommitTree) GetRootChildHash() []byte {
	return tree.root.getChildHash()
}

func (tree *CommitTree) GetRootCommitments() [][]byte {
	return tree.root.getCommitments()
}

func (tree *CommitTree) GenerateMembershipProof(val int, uid int, preferLeft bool) *CommitMembershipProof {
	leaves := make([]*commitLeafNode, 0)
	leaves = tree.root.getLeaves(val, &leaves)
	if len(leaves) == 0 {
		return nil
	}
	var leaf *commitLeafNode
	if uid == -1 { // uid of -1 means that the user id does not matter, but rather the position within the tree (e.g., for range queries)
		if preferLeft {
			leaf = leaves[0]
		} else {
			leaf = leaves[len(leaves)-1]
		}
	} else {
		for _, potentialLeaf := range leaves {
			// fmt.Println(strconv.Itoa(potentialLeaf.uid) + " vs " + strconv.Itoa(uid))
			if potentialLeaf.uid == uid {
				leaf = potentialLeaf
			}
		}
	}
	if leaf == nil {
		return nil
	}
	var cNode commitNode
	leafCopy := leaf.copy().toLeafNode()
	cNode = leaf
	cNodeHash := leaf.getHash()
	topNodeHash := tree.root.getLeftChild().getHash()
	copathNodes := make([]*forNodeOnCommitCopath, 0)
	isLeftSibling := make([]bool, 0)
	for !reflect.DeepEqual(cNodeHash, topNodeHash) {
		pNode := cNode.getParent()
		var lNodeHash, rNodeHash [32]byte
		var copathNode commitNode
		lNodeHash = pNode.getLeftChild().getHash()
		rNodeHash = pNode.getRightChild().getHash()
		if reflect.DeepEqual(cNodeHash, lNodeHash) {
			copathNode = pNode.getRightChild()
			isLeftSibling = append(isLeftSibling, false)
		} else if reflect.DeepEqual(cNodeHash, rNodeHash) { // sanity check
			copathNode = pNode.getLeftChild()
			isLeftSibling = append(isLeftSibling, true)
		} else {
			fmt.Println("error: dummy else statement reached in GenerateMembershipProof")
			log.Fatal()
		}
		node := &forNodeOnCommitCopath{
			NumLeaves:   copathNode.getNumLeaves(),
			Commitments: parseToForNodeCommitments(copathNode.getCommitments()),
			ChildHash:   copathNode.getChildHash(),
			Hash:        copathNode.getHash(),
		}
		copathNodes = append(copathNodes, node)
		cNode = pNode
		cNodeHash = cNode.getHash()
	}

	// add the root
	copathNodes = append(copathNodes, nil)
	isLeftSibling = append(isLeftSibling, false)

	// create path as struct
	path := &CommitMembershipProof{
		LeafCommitments:     parseToForNodeCommitments(leafCopy.commitments),
		LeafIdHash:          leafCopy.idHash,
		LeafHash:            leafCopy.hash,
		Copath:              copathNodes,
		CopathIsLeftSibling: isLeftSibling,
	}

	return path
}

func (p *CommitMembershipProof) RebuildRootHash() [32]byte {
	n := p.GetSize()
	var node commitNode
	node = &commitLeafNode{
		commitments: parseFromForNodeCommitments(p.LeafCommitments),
		idHash:      p.LeafIdHash,
		hash:        p.LeafHash,
	}
	for i := 0; i < n; i++ {
		var copathNode *commitInternalNode
		if p.Copath[i] != nil {
			copathNode = &commitInternalNode{
				numLeaves:   p.Copath[i].NumLeaves,
				commitments: parseFromForNodeCommitments(p.Copath[i].Commitments),
				childHash:   p.Copath[i].ChildHash,
				hash:        p.Copath[i].Hash,
			}
		} else {
			copathNode = &commitInternalNode{
				numLeaves: 0,
			}
		}
		var (
			newL   int
			nodeC  [][]byte
			childH []byte
			newH   [32]byte
		)
		if p.CopathIsLeftSibling[i] {
			newL, nodeC, childH, newH = GetInternalNodeParams(copathNode, node)
		} else {
			newL, nodeC, childH, newH = GetInternalNodeParams(node, copathNode)
		}

		node = &commitInternalNode{
			commitments: nodeC,
			hash:        newH,
			childHash:   childH,
			numLeaves:   newL,
		}
	}
	return node.getHash()
}

func (node *commitInternalNode) getLeaves(val int, leaves *[]*commitLeafNode) []*commitLeafNode {
	leftChild := node.getLeftChild()
	rightChild := node.getRightChild()

	if leftChild != nil {
		if leftChild.isLeafNode() && leftChild.getValues()[0] == val {
			*leaves = append(*leaves, leftChild.toLeafNode())
		} else if !leftChild.isLeafNode() {
			leftChild.toInternalNode().getLeaves(val, leaves)
		}
	}
	if rightChild != nil {
		if rightChild.isLeafNode() && rightChild.getValues()[0] == val {
			*leaves = append(*leaves, rightChild.toLeafNode())
		} else if !rightChild.isLeafNode() {
			rightChild.toInternalNode().getLeaves(val, leaves)
		}
	}
	return *leaves
}

// func (node *commitInternalNode) getLeftmostLeaf() *commitLeafNode {
// 	if node.getLeftChild() != nil {
// 		if node.getLeftChild().isLeafNode() {
// 			return node.getLeftChild().toLeafNode()
// 		}
// 		return node.getLeftChild().toInternalNode().GetLeftmostLeaf()
// 	}
// 	if node.getRightChild() != nil {
// 		if node.getRightChild().isLeafNode() {
// 			return node.getRightChild().toLeafNode()
// 		}
// 		return node.getRightChild().toInternalNode().GetLeftmostLeaf()
// 	}
// 	return nil
// }

// func (node *commitInternalNode) getRightmostLeaf() *commitLeafNode {
// 	if node.getRightChild() != nil {
// 		if node.getRightChild().isLeafNode() {
// 			return node.getRightChild().toLeafNode()
// 		}
// 		return node.getRightChild().toInternalNode().GetLeftmostLeaf()
// 	}
// 	if node.getLeftChild() != nil {
// 		if node.getLeftChild().isLeafNode() {
// 			return node.getLeftChild().toLeafNode()
// 		}
// 		return node.getLeftChild().toInternalNode().GetLeftmostLeaf()
// 	}
// 	return nil
// }

func addRootToTopNode(topNode commitNode) *commitInternalNode {
	root := &commitInternalNode{
		leftChild: topNode,
	}
	root.updateInternalData()
	topNode.setParent(root)
	return root
}

// func parseCommitment(comm []byte) *p256.P256 {
// 	var c *p256.P256
// 	_ = json.Unmarshal(comm, &c)
// 	return c
// }

// func checkSubTree(node commitNode, prefix string) {
// 	if node.getLeftChild() != nil {
// 		if node.getLeftChild().getParent() != node {
// 			fmt.Println("error! parent-child relationship")
// 		}
// 		checkSubTree(node.getLeftChild(), prefix+"0")
// 	}
// 	if node.getRightChild() != nil {
// 		if node.getRightChild().getParent() != node {
// 			fmt.Println("error! parent-child relationship")
// 		}
// 		checkSubTree(node.getRightChild(), prefix+"1")
// 	}

// 	if !node.isLeafNode() {
// 		newL, nodeC, childH, newH := GetInternalNodeParams(node.getLeftChild(), node.getRightChild())
// 		if newL != node.getNumLeaves() {
// 			fmt.Println("error! num leaves, " + prefix + ": " + strconv.Itoa(newL) + " vs" + strconv.Itoa(node.getNumLeaves()))
// 		}
// 		if !reflect.DeepEqual(nodeC, node.getCommitment()) {
// 			fmt.Println("error! commitment, " + prefix)
// 		}
// 		if !reflect.DeepEqual(childH, node.getChildHash()) {
// 			fmt.Println("error! child hash, " + prefix)
// 		}
// 		if !reflect.DeepEqual(newH, node.getHash()) {
// 			fmt.Println("error! hash, " + prefix)
// 		}
// 	}
// }

// func traceSubTree(node commitNode, prefix string) {
// 	fmt.Print(prefix + ": ")
// 	fmt.Print(node.getHash())
// 	fmt.Print(", n. leaves: ")
// 	fmt.Println(node.getNumLeaves())
// 	if node.getLeftChild() != nil {
// 		traceSubTree(node.getLeftChild(), prefix+"0")
// 	}
// 	if node.getRightChild() != nil {
// 		traceSubTree(node.getRightChild(), prefix+"1")
// 	}
// }

// return the root node
func BuildMerkleCommitmentTree(values, seeds [][]int, users, times []int) *CommitTree {
	// fmt.Println("***")
	// initialize the leaves
	nn := len(values)
	if nn == 0 {
		return nil
	}
	leaves := make([]commitNode, nn)
	// fmt.Println("---")
	for k := 0; k < nn; k++ {
		n := newCommitLeaf(values[k], seeds[k], users[k], times[k])
		leaves[k] = n
	}
	topNode := buildUpTreePerLayer(leaves)
	root := addRootToTopNode(topNode)
	// traceSubTree(root, "")
	// checkSubTree(root, "")
	return &CommitTree{
		root: root.toInternalNode(),
	}
}

// return the root node
func BuildMerkleCommitmentTreeFromCommitments(commitments [][]*p256.P256, idHashes [][32]byte) *CommitTree {
	// initialize the leaves
	nn := len(commitments)
	if nn == 0 {
		return nil
	}
	leaves := make([]commitNode, nn)
	// fmt.Println("---")
	for k := 0; k < nn; k++ {
		n := newCommitLeafFromCommitments(commitments[k], idHashes[k])
		leaves[k] = n
	}
	topNode := buildUpTreePerLayer(leaves)
	root := addRootToTopNode(topNode)
	return &CommitTree{
		root: root.toInternalNode(),
	}
}
