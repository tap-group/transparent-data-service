package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/tap-group/tdsvc/crypto/bulletproofs"
	"github.com/tap-group/tdsvc/crypto/p256"
	"github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/trees"
	"github.com/tap-group/tdsvc/util"
)

const DUMMY = 10 // needed because p256 does not support operations with seed 0

type Client struct {
	server server.IServer

	// bandwidth use
	bandwidth int

	prefixSendRequestTime     int64
	prefixReceiveResponseTime int64
	querySendRequestTime      int64
	queryReceiveResponseTime  int64
	finishTime                int64
}

func (client *Client) GetLatestTimestamps() (int64, int64, int64, int64, int64) {
	return client.prefixSendRequestTime, client.prefixReceiveResponseTime, client.querySendRequestTime, client.queryReceiveResponseTime, client.finishTime
}

func (client *Client) GetProcessingTime() int64 {
	return client.finishTime - client.queryReceiveResponseTime
}

func (client *Client) ResetTimestamps() {
	client.prefixSendRequestTime = time.Now().UnixNano()
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	client.querySendRequestTime = time.Now().UnixNano()
	client.queryReceiveResponseTime = time.Now().UnixNano()
	client.finishTime = time.Now().UnixNano()
}

func (client *Client) ResetBandwidthUse() {
	client.bandwidth = 0
}

func (client *Client) addBandwidthUse(k int) {
	client.bandwidth += k
}

func (client *Client) GetBandwidthUse() int {
	return client.bandwidth
}

func (client *Client) GetServer() server.IServer {
	return client.server
}

func (client *Client) SetServer(s server.IServer) {
	client.server = s
}

func removeNode(s []*trees.SetCompletenessProofNode, i int) []*trees.SetCompletenessProofNode {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func getBandwidthCostOfByteArrays(arrays [][]byte) int {
	result := 0
	for _, array := range arrays {
		result += len(array)
	}
	return result
}

func getBandwidthCostOfPrefixMembershipProof(proof *trees.MembershipProof, leafValue []byte) int {
	proofBytes, err := json.Marshal(proof)
	util.Check(err)
	return len(proofBytes) + len(leafValue)
}

func getBandwidthCostOfCommitProof(proof *trees.CommitMembershipProof) int {
	proofBytes, err := json.Marshal(proof)
	util.Check(err)
	return len(proofBytes)
}

func getBandwidthCostOfCommitProofs(proofs []*trees.CommitMembershipProof) int {
	result := 0
	for _, proof := range proofs {
		result += getBandwidthCostOfCommitProof(proof)
	}
	return result
}

func getBandwidthCostOfRangeProof(proof *bulletproofs.ProofBPRP) int {
	proofBytes, err := json.Marshal(proof)
	util.Check(err)
	return len(proofBytes)
}

func getBandwidthCostOfRangeProofs(proofs []*bulletproofs.ProofBPRP) int {
	result := 0
	for _, proof := range proofs {
		result += getBandwidthCostOfRangeProof(proof)
	}
	return result
}

func getBandwidthCostOfSetCompletenessProof(prefix [][]byte, treeRoots [][]byte, proof *trees.SetCompletenessProofNode) int {
	proofBytes, err := proof.Serialize()
	util.Check(err)
	return getBandwidthCostOfByteArrays(prefix) + getBandwidthCostOfByteArrays(treeRoots) + len(proofBytes)
}

func getBandwidthCostOfSumProof(valSum, seedSum int, commitments [][]byte, counts []int, hashes [][]byte) int {
	return 8 + getBandwidthCostOfByteArrays(commitments) + 4*len(counts) + getBandwidthCostOfByteArrays(hashes)
}

func getBandwidthCostOfMinMaxProof(minVal, argminVal int, minProof *bulletproofs.ProofBPRP, proofs []*bulletproofs.ProofBPRP, inclusionProofs []*trees.CommitMembershipProof) int {
	return 8 + getBandwidthCostOfRangeProof(minProof) + getBandwidthCostOfRangeProofs(proofs) + getBandwidthCostOfCommitProofs(inclusionProofs)
}

func getBandwidthCostOfQuantileProof(quantile int, inclusionProofsLeft []*trees.CommitMembershipProof, rangeProofsLeft []*bulletproofs.ProofBPRP, inclusionProofsRight []*trees.CommitMembershipProof, rangeProofsRight []*bulletproofs.ProofBPRP) int {
	return 4 + getBandwidthCostOfRangeProofs(rangeProofsLeft) + getBandwidthCostOfCommitProofs(inclusionProofsLeft) + getBandwidthCostOfRangeProofs(rangeProofsRight) + getBandwidthCostOfCommitProofs(inclusionProofsRight)
}

func (client *Client) LookupQuery(uid uint32, timeslot uint32, data uint32, seed uint32, miscVals []uint32) (bool, error) {
	l := len(miscVals)
	vals := make([]uint32, l+1)
	vals[0] = timeslot
	for i := 0; i < l; i++ {
		vals[i+1] = miscVals[i]
	}
	prefix := util.ValsToPrefix(vals)

	rootHash := client.server.GetPrefixTreeRootHash() // normally, the client would request this from a public bulletin board to detect possible equivoaction
	client.prefixSendRequestTime = time.Now().UnixNano()
	outputBytes := client.server.RequestPrefixMembershipProof(prefix)
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	var output *server.PrefixMembershipProofOutput
	json.Unmarshal(outputBytes, &output)
	proof, leafValue := output.Parse()
	client.addBandwidthUse(getBandwidthCostOfPrefixMembershipProof(proof, leafValue))

	fmt.Print("# valid prefix: ")
	fmt.Println(proof != nil)

	if proof != nil {
		// first check whether the proof is about the correct root hash
		rebuiltPrefixHash := proof.RebuildRootHash(leafValue)
		prefixHashValid := reflect.DeepEqual(rootHash, rebuiltPrefixHash)
		if !prefixHashValid {
			client.ResetTimestamps()
			return false, errors.New("invalid prefix tree inclusion proof")
		}

		// request commitment tree membership proof
		input := server.NewCommitmentMembershipProofInput(prefix, data, uid)
		inputBytes, err := json.Marshal(input)
		util.Check(err)
		client.querySendRequestTime = time.Now().UnixNano()
		outputBytes = client.server.RequestCommitmentTreeMembershipProof(inputBytes)
		client.queryReceiveResponseTime = time.Now().UnixNano()
		var output *server.ByteSlice
		json.Unmarshal(outputBytes, &output)

		client.addBandwidthUse(output.Size())
		commitProof := output.ParseToCommitMembershipProof()
		if commitProof == nil {
			client.ResetTimestamps()
			return false, nil
		}

		rebuiltCommitHash := commitProof.RebuildRootHash()
		commitHashValid := reflect.DeepEqual(leafValue, rebuiltCommitHash[:])
		if !commitHashValid {
			client.ResetTimestamps()
			return false, errors.New("invalid prefix tree inclusion proof")
		}
		client.finishTime = time.Now().UnixNano()
		return true, nil

	}
	client.finishTime = time.Now().UnixNano()
	return false, nil
}

func verifyPrefixSetCompletenessProof(valRange [][]uint32, prefixes [][]byte, proof *trees.SetCompletenessProofNode, treeRoots [][]byte, rootHash []byte) bool {
	// first check if the hashes in the completeness proof are correct and the root hash is correct
	// fmt.Println("verifying set completeness proof!")
	internallyValid := proof.CheckHash()
	if !internallyValid {
		return false
	}
	rootHashCorrect := reflect.DeepEqual(proof.GetHash(), rootHash)
	if !rootHashCorrect {
		return false
	}

	// finally, obtain the valid leaf nodes in the proof
	validLeafNodes := make([]*trees.SetCompletenessProofNode, 0)
	validLeafNodes = trees.FindValidNodes(proof, validLeafNodes, make([]byte, 0), valRange, 4)
	// nInit := len(validLeafNodes)

	// check for a 1-1 correspondence between the valid leaf nodes and the prefixes
	// first check if the length of the two slices is the same
	if len(prefixes) != len(validLeafNodes) {
		return false
	}

	// then remove all elements from validLeafNodes for which there is a match in prefixes
	for i := range prefixes {
		for j, node := range validLeafNodes {
			h := util.Hash(node.GetPartialPrefix(), treeRoots[i])
			if reflect.DeepEqual(h, node.GetHash()) && reflect.DeepEqual(util.ConvertBytesToBits(prefixes[i]), node.GetCumulativePrefix()) {
				// fmt.Print("match! ")
				// fmt.Println(prefixes[i])
				validLeafNodes = removeNode(validLeafNodes, j)
				break
			}
		}
	}

	// after removing all matches, validLeafNodes should be empty
	// fmt.Println("prefixes unaccounted for: " + strconv.Itoa(len(validLeafNodes)) + " of " + strconv.Itoa(nInit))
	return len(validLeafNodes) == 0
}

func (client *Client) requestAndVerifySumAndCount(valRange [][]uint32) (int, int, error) {
	rootHash := client.server.GetPrefixTreeRootHash()
	valInput := server.NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	client.prefixSendRequestTime = time.Now().UnixNano()
	prefixOutputBytes := client.server.RequestPrefixes(valBytes)
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	var output *server.PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	prefixes, treeroots, setProofBytes := output.Parse()
	setProof, err := trees.DeserializeSetCompletenessProofNode(setProofBytes)
	util.Check(err)
	client.addBandwidthUse(getBandwidthCostOfSetCompletenessProof(prefixes, treeroots, setProof))
	prefixCompletenessProofValid := verifyPrefixSetCompletenessProof(valRange, prefixes, setProof, treeroots, rootHash)
	if !prefixCompletenessProofValid {
		client.ResetTimestamps()
		return -1, -1, errors.New("invalid prefix set completeness proof")
	}

	fmt.Print("# valid prefixes: ")
	fmt.Println(len(prefixes))

	H, _ := p256.MapToGroup(trees.COMMIT_SEEDH)

	client.querySendRequestTime = time.Now().UnixNano()
	valSum, seedSum, commitments, counts, hashes := client.server.RequestSumAndCounts(prefixes)
	client.queryReceiveResponseTime = time.Now().UnixNano()
	if len(commitments) > 0 {
		client.addBandwidthUse(getBandwidthCostOfSumProof(valSum, seedSum, commitments[0], counts, hashes))
	} else {
		client.addBandwidthUse(getBandwidthCostOfSumProof(valSum, seedSum, nil, counts, hashes))
	}
	var totalCommitment *p256.P256
	n := len(commitments)
	totCount := 0
	init := true
	for i := 0; i < n; i++ {
		if counts[i] > 0 {
			totCount += counts[i]
			hashComp := util.HashCommitmentsAndCount(commitments[i], counts[i])
			checkHash := util.Hash32(append(hashes[i], hashComp[:]...))

			valid := reflect.DeepEqual(treeroots[i], checkHash[:])
			if !valid {
				client.ResetTimestamps()
				return -1, -1, errors.New("invalid hash")
			}

			var newCommitment *p256.P256
			_ = json.Unmarshal(commitments[i][0], &newCommitment)

			if !init {
				totalCommitment = new(p256.P256).Add(totalCommitment, newCommitment)
			} else {
				totalCommitment = newCommitment
				init = false
			}
		}
	}

	if totalCommitment == nil {
		client.ResetTimestamps()
		fmt.Println("warning: no entries found that match the search criteria")
		return -1, -1, nil
	}

	C, _ := p256.CommitG1(big.NewInt(int64(valSum)), big.NewInt(int64(seedSum)), H)
	valid := C.Equals(totalCommitment)

	client.ResetTimestamps()

	if !valid {
		return -1, -1, errors.New("sum of commitments does not match")
	}

	client.finishTime = time.Now().UnixNano()
	return valSum, totCount, nil
}

func (client *Client) SumQuery(valRange [][]uint32) (int, error) {
	sum, _, err := client.requestAndVerifySumAndCount(valRange)
	return sum, err
}

func (client *Client) CountQuery(valRange [][]uint32) (int, error) {
	_, count, err := client.requestAndVerifySumAndCount(valRange)
	return count, err
}

func (client *Client) AvgQuery(valRange [][]uint32) (float64, error) {
	sum, count, err := client.requestAndVerifySumAndCount(valRange)
	return float64(sum) / float64(count), err
}

func verifyLeafProof(rangeProof *bulletproofs.ProofBPRP, inclusionProof *trees.CommitMembershipProof, minVal uint32, maxVal uint32) bool {
	// first check the proof itself
	valid, err := rangeProof.Verify()
	if err != nil || !valid {
		fmt.Println("invalid range proof")
		return false
	}

	// the commitments in the proof must correspond to the commitment in the leaf of the inclusion proof
	H, _ := p256.MapToGroup(trees.COMMIT_SEEDH)

	var leafCommitment *p256.P256
	_ = json.Unmarshal(inclusionProof.GetLeafCommitment(0), &leafCommitment)
	D1, err1 := p256.CommitG1(big.NewInt(int64(maxVal)-bulletproofs.MAX_RANGE_END), big.NewInt(DUMMY), H) // in leaf should be x - b + 2^N, so add b - 2^N to obtain x
	D2, err2 := p256.CommitG1(big.NewInt(int64(minVal)), big.NewInt(DUMMY), H)                            // in leaf should be x - a, so add a to obtain x
	Dnil, err3 := p256.CommitG1(big.NewInt(0), big.NewInt(DUMMY), H)
	if !(err1 == nil && err2 == nil && err3 == nil) {
		return false
	}

	Comm1Dummy := new(p256.P256).Add(rangeProof.P1.V, D1)
	Comm2Dummy := new(p256.P256).Add(rangeProof.P2.V, D2)
	leafDummy := new(p256.P256).Add(leafCommitment.Copy(), Dnil)

	ok1 := Comm1Dummy.Equals(leafDummy)
	ok2 := Comm2Dummy.Equals(leafDummy)
	if !ok1 {
		fmt.Println("commitment mismatch: 1")
	}
	if !ok2 {
		fmt.Println("commitment mismatch: 2")
	}
	return ok1 && ok2
}

func verifyInclusionProof(proof *trees.CommitMembershipProof, rootHash []byte, isLeftSibling []bool) bool {
	if isLeftSibling != nil {
		n := len(proof.Copath)
		for i := 0; i < n-1; i++ {
			if proof.CopathIsLeftSibling[i] != isLeftSibling[i] {
				fmt.Println("error: left/right sequence mismatch")
				fmt.Println(proof.CopathIsLeftSibling)
				fmt.Println(isLeftSibling)
				fmt.Println(proof)
				return false
			}
		}
		// top node below root can never be the left sibling
		if proof.CopathIsLeftSibling[n-1] {
			fmt.Println("error: left/right sequence mismatch")
			fmt.Println(proof.CopathIsLeftSibling)
			fmt.Println(isLeftSibling)
			return false
		}

	}
	rebuiltRootHash := proof.RebuildRootHash()
	equal := reflect.DeepEqual(rootHash, rebuiltRootHash[:])
	if !equal {
		fmt.Println("error: rebuilt root hash does not match")
		fmt.Println(rootHash)
		fmt.Println(rebuiltRootHash[:])
	}
	return equal
}

func (client *Client) requestAndVerifyMin(valRange [][]uint32) (int, error) {
	rootHash := client.server.GetPrefixTreeRootHash()
	valInput := server.NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	client.prefixSendRequestTime = time.Now().UnixNano()
	prefixOutputBytes := client.server.RequestPrefixes(valBytes)
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	var output *server.PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	prefixes, treeroots, setProofBytes := output.Parse()
	setProof, err := trees.DeserializeSetCompletenessProofNode(setProofBytes)
	util.Check(err)
	client.addBandwidthUse(getBandwidthCostOfSetCompletenessProof(prefixes, treeroots, setProof))
	prefixCompletenessProofValid := verifyPrefixSetCompletenessProof(valRange, prefixes, setProof, treeroots, rootHash)
	if !prefixCompletenessProofValid {
		client.ResetTimestamps()
		return -1, errors.New("invalid prefix set completeness proof")
	}

	n := len(prefixes)

	fmt.Print("# valid prefixes: ")
	fmt.Println(len(prefixes))

	prefixInput := server.NewPrefixSet(prefixes)
	prefixInputBytes, err := json.Marshal(prefixInput)
	util.Check(err)
	client.querySendRequestTime = time.Now().UnixNano()
	minVal, argminVal, minProofBytes, rangeProofBytes, inclusionProofBytes := client.server.RequestMinAndProofs(prefixInputBytes)
	client.queryReceiveResponseTime = time.Now().UnixNano()
	var minProof *bulletproofs.ProofBPRP
	json.Unmarshal(minProofBytes, &minProof)
	rangeProofs := make([]*bulletproofs.ProofBPRP, len(rangeProofBytes))
	inclusionProofs := make([]*trees.CommitMembershipProof, len(inclusionProofBytes))
	for i, proofBytes := range rangeProofBytes {
		var proof bulletproofs.ProofBPRP
		json.Unmarshal(proofBytes, &proof)
		rangeProofs[i] = &proof
	}
	for i, proofBytes := range inclusionProofBytes {
		var proof trees.CommitMembershipProof
		json.Unmarshal(proofBytes, &proof)
		inclusionProofs[i] = &proof
	}

	client.addBandwidthUse(getBandwidthCostOfMinMaxProof(minVal, argminVal, minProof, rangeProofs, inclusionProofs))

	fmt.Println("checking proofs... ")

	// check validity of the inclusion proofs
	var wg sync.WaitGroup
	valids := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			// for the min, all the siblings in the inclusion proof's copath must be on the right, so isLeftSibling must be false everywhere
			m := len(inclusionProofs[k].Copath)
			isLeftSibling := make([]bool, m)
			for j := 0; j < m; j++ {
				isLeftSibling[j] = false
			}
			valids[k] = verifyInclusionProof(inclusionProofs[k], treeroots[k], isLeftSibling)
		}(i)
	}

	wg.Wait()
	for i := 0; i < len(valids); i++ {
		if !valids[i] {
			client.ResetTimestamps()
			return -1, errors.New("invalid inclusion proof for a commitment tree leaf")
		}
	}

	if argminVal == -1 {
		fmt.Println("warning: no entries found that match the search criteria")
		client.ResetTimestamps()
		return -1, nil
	}

	// check validity of the range proofs
	minProofValid := verifyLeafProof(minProof, inclusionProofs[argminVal], uint32(minVal), uint32(minVal)+1)
	if !minProofValid {
		client.ResetTimestamps()
		return -1, errors.New("invalid range proof for the total min")
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			valids[k] = verifyLeafProof(rangeProofs[k], inclusionProofs[k], uint32(minVal), server.BP_MAX)
		}(i)
	}

	wg.Wait()
	for i := 0; i < len(valids); i++ {
		if !valids[i] {
			client.ResetTimestamps()
			return -1, errors.New("invalid range proof for a commitment tree min")
		}
	}

	// H, _ := p256.MapToGroup(merklecommittree.MERKLE_SEEDH)

	client.finishTime = time.Now().UnixNano()
	return minVal, nil
}

func (client *Client) MinQuery(valRange [][]uint32) (int, error) {
	min, err := client.requestAndVerifyMin(valRange)
	return min, err
}

func (client *Client) requestAndVerifyMax(valRange [][]uint32) (int, error) {
	rootHash := client.server.GetPrefixTreeRootHash()
	valInput := server.NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	client.prefixSendRequestTime = time.Now().UnixNano()
	prefixOutputBytes := client.server.RequestPrefixes(valBytes)
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	var output *server.PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	prefixes, treeroots, setProofBytes := output.Parse()
	setProof, err := trees.DeserializeSetCompletenessProofNode(setProofBytes)
	util.Check(err)
	prefixCompletenessProofValid := verifyPrefixSetCompletenessProof(valRange, prefixes, setProof, treeroots, rootHash)
	client.addBandwidthUse(getBandwidthCostOfSetCompletenessProof(prefixes, treeroots, setProof))
	if !prefixCompletenessProofValid {
		client.ResetTimestamps()
		return -1, errors.New("invalid prefix set completeness proof")
	}
	n := len(prefixes)

	fmt.Print("# valid prefixes: ")
	fmt.Println(len(prefixes))

	client.querySendRequestTime = time.Now().UnixNano()
	maxVal, argmaxVal, maxProofBytes, rangeProofBytes, inclusionProofsBytes := client.server.RequestMaxAndProofs(prefixes)
	client.queryReceiveResponseTime = time.Now().UnixNano()
	var maxProof *bulletproofs.ProofBPRP
	rangeProofs := make([]*bulletproofs.ProofBPRP, len(rangeProofBytes))
	inclusionProofs := make([]*trees.CommitMembershipProof, len(inclusionProofsBytes))
	json.Unmarshal(maxProofBytes, &maxProof)
	for i, proofBytes := range rangeProofBytes {
		var proof bulletproofs.ProofBPRP
		json.Unmarshal(proofBytes, &proof)
		rangeProofs[i] = &proof
	}
	for i, proofBytes := range inclusionProofsBytes {
		var proof trees.CommitMembershipProof
		json.Unmarshal(proofBytes, &proof)
		inclusionProofs[i] = &proof
	}
	client.addBandwidthUse(getBandwidthCostOfMinMaxProof(maxVal, argmaxVal, maxProof, rangeProofs, inclusionProofs))

	fmt.Println("checking proofs... ")

	// check validity of the inclusion proofs
	var wg sync.WaitGroup
	valids := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			// for the max, all the siblings in the inclusion proof's copath must be on the right, so isLeftSibling must be true everywhere
			m := len(inclusionProofs[k].Copath)
			isLeftSibling := make([]bool, m)
			for j := 0; j < m; j++ {
				isLeftSibling[j] = true
			}
			valids[k] = verifyInclusionProof(inclusionProofs[k], treeroots[k], isLeftSibling)
		}(i)
	}

	wg.Wait()
	for i := 0; i < len(valids); i++ {
		if !valids[i] {
			return -1, errors.New("invalid inclusion proof for a commitment tree leaf")
		}
	}

	if argmaxVal == -1 {
		client.ResetTimestamps()
		fmt.Println("warning: no entries found that match the search criteria")
		return -1, nil
	}

	// check validity of the range proofs
	maxProofValid := verifyLeafProof(maxProof, inclusionProofs[argmaxVal], uint32(maxVal), uint32(maxVal)+1)
	if !maxProofValid {
		client.ResetTimestamps()
		return -1, errors.New("invalid range proof for the total max")
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			valids[k] = verifyLeafProof(rangeProofs[k], inclusionProofs[k], 0, uint32(maxVal)+1)
		}(i)
	}

	wg.Wait()
	for i := 0; i < len(valids); i++ {
		if !valids[i] {
			return -1, errors.New("invalid range proof for a commitment tree min")
		}
	}

	client.finishTime = time.Now().UnixNano()
	return maxVal, nil
}

func (client *Client) MaxQuery(valRange [][]uint32) (int, error) {
	max, err := client.requestAndVerifyMax(valRange)
	return max, err
}

func (client *Client) requestAndVerifyQuantile(valRange [][]uint32, k int, q int) (int, error) {
	rootHash := client.server.GetPrefixTreeRootHash()
	valInput := server.NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	client.prefixSendRequestTime = time.Now().UnixNano()
	prefixOutputBytes := client.server.RequestPrefixes(valBytes)
	client.prefixReceiveResponseTime = time.Now().UnixNano()
	var output *server.PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	prefixes, treeroots, setProofBytes := output.Parse()
	setProof, err := trees.DeserializeSetCompletenessProofNode(setProofBytes)
	util.Check(err)
	client.addBandwidthUse(getBandwidthCostOfSetCompletenessProof(prefixes, treeroots, setProof))
	prefixCompletenessProofValid := verifyPrefixSetCompletenessProof(valRange, prefixes, setProof, treeroots, rootHash)
	if !prefixCompletenessProofValid {
		client.ResetTimestamps()
		return -1, errors.New("invalid prefix set completeness proof")
	}

	n := len(prefixes)

	fmt.Print("# valid prefixes: ")
	fmt.Println(n)

	client.querySendRequestTime = time.Now().UnixNano()
	quantile, _, inclusionProofsLeftBytes, rangeProofsLeftBytes, inclusionProofsRightBytes, rangeProofsRightBytes := client.server.RequestQuantileAndProofs(prefixes, k, q)
	client.queryReceiveResponseTime = time.Now().UnixNano()
	inclusionProofsLeft := make([]*trees.CommitMembershipProof, len(inclusionProofsLeftBytes))
	rangeProofsLeft := make([]*bulletproofs.ProofBPRP, len(rangeProofsLeftBytes))
	inclusionProofsRight := make([]*trees.CommitMembershipProof, len(inclusionProofsRightBytes))
	rangeProofsRight := make([]*bulletproofs.ProofBPRP, len(rangeProofsRightBytes))
	for i, proofBytes := range inclusionProofsLeftBytes {
		var proof trees.CommitMembershipProof
		json.Unmarshal(proofBytes, &proof)
		inclusionProofsLeft[i] = &proof
	}
	for i, proofBytes := range rangeProofsLeftBytes {
		var proof bulletproofs.ProofBPRP
		json.Unmarshal(proofBytes, &proof)
		rangeProofsLeft[i] = &proof
	}

	for i, proofBytes := range inclusionProofsRightBytes {
		var proof trees.CommitMembershipProof
		json.Unmarshal(proofBytes, &proof)
		inclusionProofsRight[i] = &proof
	}
	for i, proofBytes := range rangeProofsRightBytes {
		var proof bulletproofs.ProofBPRP
		json.Unmarshal(proofBytes, &proof)
		rangeProofsRight[i] = &proof
	}
	client.addBandwidthUse(getBandwidthCostOfQuantileProof(quantile, inclusionProofsLeft, rangeProofsLeft, inclusionProofsRight, rangeProofsRight))

	leftCount := 0
	rightCount := 0
	totalCount := 0
	fmt.Print("result: ")
	fmt.Println(quantile)
	// check validity of the inclusion proofs
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	invalids := make([][]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			invalids[k] = make([]bool, 4)
			// left side
			if len(inclusionProofsLeft[k].Copath) > 0 {
				invalids[k][0] = !verifyInclusionProof(inclusionProofsLeft[k], treeroots[k], nil)
				invalids[k][1] = !verifyLeafProof(rangeProofsLeft[k], inclusionProofsLeft[k], 0, uint32(quantile)+1)

				mu.Lock()
				leftCount += inclusionProofsLeft[k].GetNumLeftNodes() + 1
				mu.Unlock()
			}
			// right side
			if len(inclusionProofsRight[k].Copath) > 0 {
				invalids[k][2] = !verifyInclusionProof(inclusionProofsRight[k], treeroots[k], nil)
				invalids[k][3] = !verifyLeafProof(rangeProofsRight[k], inclusionProofsRight[k], uint32(quantile), server.BP_MAX)
				mu.Lock()
				rightCount += inclusionProofsRight[k].GetNumRightNodes() + 1
				mu.Unlock()
			}
			// one should always exist, unless the commitment tree is empty, in which case we do not need to increment totalCount
			mu.Lock()
			if inclusionProofsLeft[k] != nil {
				totalCount += inclusionProofsLeft[k].GetTotalNumNodes()
			} else if inclusionProofsRight[k] != nil {
				totalCount += inclusionProofsRight[k].GetTotalNumNodes()
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	for i := 0; i < len(invalids); i++ {
		if invalids[i][0] {
			return -1, errors.New("invalid inclusion proof for a commitment tree leaf " + strconv.Itoa(i) + " (left)")
		}
		if invalids[i][1] {
			return -1, errors.New("invalid range proof for a commitment tree leaf " + strconv.Itoa(i) + " (left)")
		}
		if invalids[i][2] {
			return -1, errors.New("invalid inclusion proof for a commitment tree leaf " + strconv.Itoa(i) + " (right)")
		}
		if invalids[i][3] {
			return -1, errors.New("invalid range proof for a commitment tree leaf " + strconv.Itoa(i) + " (right)")
		}
	}

	fmt.Println("# total nodes: " + strconv.Itoa(totalCount) + ", left nodes: " + strconv.Itoa(leftCount) + ", right nodes: " + strconv.Itoa(rightCount))
	if leftCount < totalCount*k/q {
		client.ResetTimestamps()
		return -1, errors.New("too few nodes on the left")
	}
	if rightCount < totalCount*k/q {
		client.ResetTimestamps()
		return -1, errors.New("too few nodes on the right")
	}
	fmt.Println("quantile: " + strconv.Itoa(quantile))

	client.finishTime = time.Now().UnixNano()
	return quantile, nil
}

func (client *Client) QuantileQuery(valRange [][]uint32, k int, q int) (int, error) {
	quantile, err := client.requestAndVerifyQuantile(valRange, k, q)
	return quantile, err
}
