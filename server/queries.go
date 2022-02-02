package server

import (
	"encoding/json"

	"github.com/tap-group/tdsvc/crypto/bulletproofs"
	"github.com/tap-group/tdsvc/crypto/p256"
	"github.com/tap-group/tdsvc/trees"
	"github.com/tap-group/tdsvc/util"
)

/* prefix membership proof */

type PrefixMembershipProofOutput struct {
	ProofBytes []byte
	LeafValue  []byte
}

func NewPrefixMembershipProofOutput(proof *trees.MembershipProof, leafValue []byte) *PrefixMembershipProofOutput {
	proofBytes, _ := json.Marshal(proof)
	result := &PrefixMembershipProofOutput{
		ProofBytes: proofBytes,
		LeafValue:  leafValue,
	}
	return result
}

func (output *PrefixMembershipProofOutput) Parse() (*trees.MembershipProof, []byte) {
	var proof *trees.MembershipProof
	json.Unmarshal(output.ProofBytes, &proof)
	return proof, output.LeafValue
}

/* commitment membership proof */

type CommitmentMembershipProofInput struct {
	Prefix   []byte
	ValBytes []byte
	UidBytes []byte
}

func NewCommitmentMembershipProofInput(prefix []byte, val, uid uint32) *CommitmentMembershipProofInput {
	valBytes := util.IntToBytes(val)
	uidBytes := util.IntToBytes(uid)
	result := &CommitmentMembershipProofInput{
		Prefix:   prefix,
		ValBytes: valBytes,
		UidBytes: uidBytes,
	}
	return result
}

func (output *CommitmentMembershipProofInput) Parse() ([]byte, uint32, uint32) {
	return output.Prefix, util.BytesToInt(output.ValBytes), util.BytesToInt(output.UidBytes)
}

type ByteSlice struct {
	Slice []byte
}

func NewCommitmentMembershipProofOutput(proof *trees.CommitMembershipProof) *ByteSlice {
	proofBytes, err := json.Marshal(proof)
	util.Check(err)
	result := &ByteSlice{
		Slice: proofBytes,
	}
	return result
}

func (output *ByteSlice) ParseToCommitMembershipProof() *trees.CommitMembershipProof {
	var proof *trees.CommitMembershipProof
	json.Unmarshal(output.Slice, &proof)
	return proof
}

func (output *ByteSlice) Size() int {
	return len(output.Slice)
}

type CommitmentsAndProofsOutput struct {
	CommitmentArrays []*CommitmentArray
	IdHashArrays     []*IdHashArray
	Proofs           []*bulletproofs.ProofBPRP
}

type CommitmentArray struct {
	Commitments []*p256.P256
}

type IdHashArray struct {
	IdHashes [32]byte
}

func NewCommitmentsAndProofs(commitments [][]*p256.P256, idHashes [][32]byte, proofs []*bulletproofs.ProofBPRP) *CommitmentsAndProofsOutput {
	n := len(commitments)
	Commitments := make([]*CommitmentArray, n)
	IdHashes := make([]*IdHashArray, n)
	for i := range commitments {
		Commitments[i] = &CommitmentArray{
			Commitments: commitments[i],
		}
		IdHashes[i] = &IdHashArray{
			IdHashes: idHashes[i],
		}
	}
	return &CommitmentsAndProofsOutput{
		CommitmentArrays: Commitments,
		IdHashArrays:     IdHashes,
		Proofs:           proofs,
	}
}

func (output *CommitmentsAndProofsOutput) Parse() ([][]*p256.P256, [][32]byte, []*bulletproofs.ProofBPRP) {
	n := len(output.CommitmentArrays)
	commitments := make([][]*p256.P256, n)
	idHashes := make([][32]byte, n)
	for i := range commitments {
		commitments[i] = output.CommitmentArrays[i].Commitments
		idHashes[i] = output.IdHashArrays[i].IdHashes
	}
	return commitments, idHashes, output.Proofs
}

type PrefixesInput struct {
	ValRange []*RangeInput
}

type RangeInput struct {
	Range []uint32
}

func NewPrefixesInput(valRange [][]uint32) *PrefixesInput {
	ranges := make([]*RangeInput, len(valRange))
	for i, a := range valRange {
		ranges[i] = &RangeInput{
			Range: a,
		}
	}
	return &PrefixesInput{
		ValRange: ranges,
	}
}

func (input *PrefixesInput) Parse() [][]uint32 {
	n := len(input.ValRange)
	result := make([][]uint32, n)
	for i := range input.ValRange {
		result[i] = input.ValRange[i].Range
	}
	return result
}

type PrefixesOutput struct {
	ValidPrefixes   []*ByteSlice
	ValidTreeRoots  []*ByteSlice
	SerializedProof []byte
}

func NewPrefixesOutput(validPrefixes [][]byte, validTreeRoots [][]byte, serializedProof []byte) *PrefixesOutput {
	n := len(validPrefixes)
	ValidPrefixes := make([]*ByteSlice, n)
	ValidTreeRoots := make([]*ByteSlice, n)
	for i := range validPrefixes {
		ValidPrefixes[i] = &ByteSlice{Slice: validPrefixes[i]}
		ValidTreeRoots[i] = &ByteSlice{Slice: validTreeRoots[i]}
	}
	return &PrefixesOutput{
		ValidPrefixes:   ValidPrefixes,
		ValidTreeRoots:  ValidTreeRoots,
		SerializedProof: serializedProof,
	}
}

func (output *PrefixesOutput) Parse() ([][]byte, [][]byte, []byte) {
	validPrefixes := make([][]byte, len(output.ValidPrefixes))
	validTreeRoots := make([][]byte, len(output.ValidPrefixes))
	for i := range output.ValidPrefixes {
		validPrefixes[i] = output.ValidPrefixes[i].Slice
		validTreeRoots[i] = output.ValidTreeRoots[i].Slice
	}
	return validPrefixes, validTreeRoots, output.SerializedProof
}

type PrefixSet struct {
	Prefixes []*ByteSlice
}

func NewPrefixSet(a [][]byte) *PrefixSet {
	n := len(a)
	result := make([]*ByteSlice, n)
	for i := range a {
		result[i] = &ByteSlice{Slice: a[i]}
	}
	return &PrefixSet{Prefixes: result}
}

func (set *PrefixSet) Parse() [][]byte {
	result := make([][]byte, len(set.Prefixes))
	for i, p := range set.Prefixes {
		result[i] = p.Slice
	}
	return result
}
