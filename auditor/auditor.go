package auditor

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/tap-group/tdsvc/crypto/bulletproofs"
	"github.com/tap-group/tdsvc/crypto/p256"
	"github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/trees"
	"github.com/tap-group/tdsvc/util"
)

const DUMMY = 10 // needed because p256 does not support operations with seed 0

type Auditor struct {
	server server.IServer

	//built locally
	prefixTree *trees.PrefixTree

	//transmitted
	prefixes            [][]byte
	commitmentTreeRoots [][]byte

	// bandwidth use
	bandwidth int
}

func (auditor *Auditor) SetServer(s server.IServer) {
	auditor.server = s
}

func (auditor *Auditor) GetStorageCost() int {
	// non-trivial fields: prefix tree, prefixes, commitment tree roots
	prefixTreeBytes, _ := auditor.prefixTree.Serialize()
	totalSize := len(prefixTreeBytes)
	for _, prefix := range auditor.prefixes {
		totalSize += len(prefix)
	}
	for _, commitment := range auditor.commitmentTreeRoots {
		totalSize += len(commitment)
	}
	return totalSize
}

func (auditor *Auditor) ResetBandwidthUse() {
	auditor.bandwidth = 0
}

func (auditor *Auditor) GetBandwidthUse() int {
	return auditor.bandwidth
}

func (auditor *Auditor) RequestAndBuildPrefixTree(valRange [][]uint32) {
	auditor.prefixTree = trees.NewPrefixTree()
	valInput := server.NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	prefixOutputBytes := auditor.server.RequestPrefixes(valBytes)
	var output *server.PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	auditor.prefixes, auditor.commitmentTreeRoots, _ = output.Parse()

	n := len(auditor.prefixes)

	for i := 0; i < n; i++ {
		bitPrefix := util.ConvertBytesToBits(auditor.prefixes[i])
		auditor.prefixTree.PrefixAppend(bitPrefix, auditor.commitmentTreeRoots[i])
	}

	// bandwidth cost: prefixes & commitmentTreeRoots
	for _, prefix := range auditor.prefixes {
		auditor.bandwidth += len(prefix)
	}
	for _, commitment := range auditor.commitmentTreeRoots {
		auditor.bandwidth += len(commitment)
	}
}

func (auditor *Auditor) CheckPrefixTree(nColumns int) bool {
	valRange := make([][]uint32, nColumns)
	for i := range valRange {
		valRange[i] = []uint32{0, math.MaxInt32}
	}

	rootHash := auditor.server.GetPrefixTreeRootHash()
	auditor.RequestAndBuildPrefixTree(valRange)

	return reflect.DeepEqual(rootHash, auditor.prefixTree.GetHash())
}

func (auditor *Auditor) CheckAllCommitTrees(valRange [][]uint32) bool {
	auditor.RequestAndBuildPrefixTree(valRange)

	n := len(auditor.prefixes)
	for i := 0; i < n; i++ {
		fmt.Print("tree ")
		fmt.Print(i)
		fmt.Print("/")
		fmt.Print(n)
		fmt.Print(", ")
		valid := auditor.CheckCommitTree(auditor.prefixes[i], auditor.commitmentTreeRoots[i])
		if !valid {
			return false
		}
	}
	return true
}

func (auditor *Auditor) CheckCommitTree(prefix []byte, commitmentTreeRootHash []byte) bool {
	outputBytes := auditor.server.RequestCommitmentsAndProofs(prefix)
	var output *server.CommitmentsAndProofsOutput
	json.Unmarshal(outputBytes, &output)
	commitments, idHashes, proofs := output.Parse()
	tree := trees.BuildMerkleCommitmentTreeFromCommitments(commitments, idHashes)

	rootHashValid := auditor.CheckCommitTreeRootHash(tree, commitmentTreeRootHash)
	rangeProofsValid := auditor.CheckRangeProofs(commitments, proofs)

	// bandwidth cost: commitments, idHashes, and proofs
	for _, commitment := range commitments {
		commitmentBytes, err := json.Marshal(commitment)
		util.Check(err)
		auditor.bandwidth += len(commitmentBytes)
	}
	for _, idHash := range idHashes {
		auditor.bandwidth += len(idHash)
	}
	for _, proof := range proofs {
		proofBytes, err := json.Marshal(proof)
		util.Check(err)
		auditor.bandwidth += len(proofBytes)
	}

	return rootHashValid && rangeProofsValid
}

func (auditor *Auditor) CheckCommitTreeRootHash(tree *trees.CommitTree, commitmentTreeRootHash []byte) bool {
	if tree != nil {
		rootHash := tree.GetRootHash()
		return reflect.DeepEqual(rootHash[:], commitmentTreeRootHash)
	}

	nilHash := util.NilHash()
	return reflect.DeepEqual(nilHash[:], commitmentTreeRootHash)
}

func (auditor *Auditor) CheckRangeProofs(commitments [][]*p256.P256, proofs []*bulletproofs.ProofBPRP) bool {
	n := len(proofs)
	fmt.Print("checking ")
	fmt.Print(n)
	fmt.Println(" proofs")

	H, _ := p256.MapToGroup(trees.COMMIT_SEEDH)

	for i := 0; i < n; i++ {
		negComm := commitments[i][0].Copy()
		diffComm := commitments[i+1][0].Copy()
		negComm = negComm.Neg(negComm)
		diffComm = diffComm.Add(diffComm, negComm)

		D1, err1 := p256.CommitG1(big.NewInt(0), big.NewInt(DUMMY), H)
		util.Check(err1)
		D2, err2 := p256.CommitG1(big.NewInt(bulletproofs.MAX_RANGE_END-server.BP_MAX), big.NewInt(DUMMY), H)
		util.Check(err2)
		P1Vdummy := new(p256.P256).Add(proofs[i].P1.V, D1)
		Comm1Dummy := new(p256.P256).Add(diffComm, D2)

		ok1 := P1Vdummy.Equals(Comm1Dummy)
		ok2 := proofs[i].P2.V.Equals(diffComm)
		ok3, err3 := proofs[i].Verify()
		util.Check(err3)

		if !(ok1 && ok2 && ok3) {
			return false
		}
	}
	return true
}
