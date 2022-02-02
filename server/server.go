package server

import (
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"github.com/tap-group/tdsvc/crypto/bulletproofs"
	"github.com/tap-group/tdsvc/crypto/p256"
	"github.com/tap-group/tdsvc/trees"
	"github.com/tap-group/tdsvc/util"
)

const BP_MAX = 10000000 // maximum value in the column with index dataRowIdx
const N_MOMENTS = 3     // 1 for just sums, 2 for sum of squares, 3 for sum of cubes, etc

type Server struct {
	columnNames []string
	tableName   string
	userRowIdx,
	timeRowIdx,
	dataRowIdx,
	seedRowIdx uint32
	miscFieldsIdxs []uint32

	prefixTree      *trees.PrefixTree
	prefixes        [][]byte
	commitmentTrees [][]byte

	prefixReceiveRequestTime int64
	prefixSendResponseTime   int64
	queryReceiveRequestTime  int64
	querySendResponseTime    int64

	sqlDuration             int64
	buildCommitmentDuration int64
}

func (server *Server) GetSqlDuration() int64 {
	return server.sqlDuration
}

func (server *Server) GetBuildCommitmentDuration() int64 {
	return server.buildCommitmentDuration
}

func (server *Server) ResetDurations() {
	server.sqlDuration = 0
	server.buildCommitmentDuration = 0
}

func (server *Server) GetLatestTimestamps() (int64, int64, int64, int64) {
	return server.prefixReceiveRequestTime, server.prefixSendResponseTime, server.queryReceiveRequestTime, server.querySendResponseTime
}

func (server *Server) ResetTimestamps() {
	server.prefixReceiveRequestTime = time.Now().UnixNano()
	server.prefixSendResponseTime = time.Now().UnixNano()
	server.queryReceiveRequestTime = time.Now().UnixNano()
	server.querySendResponseTime = time.Now().UnixNano()
}

func LoadTable(filename string) ([][]int, []string) {
	n := 0
	m := 0
	file, err := os.Open(filename)
	util.Check(err)
	r := csv.NewReader(file)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {
			util.Check(err)
		}
		n += 1
		m = len(record)
	}

	result := make([][]int, m)
	for i := range result {
		result[i] = make([]int, n-1)
	}
	col_names := make([]string, m)

	count := -1
	file, err = os.Open(filename)
	util.Check(err)
	r = csv.NewReader(file)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {
			util.Check(err)
		}

		if count >= 0 {
			for j := range record {
				result[j][count], err = strconv.Atoi(record[j])
				util.Check(err)
			}
		} else {
			for j := range record {
				col_names[j] = record[j]
				util.Check(err)

			}
		}

		count += 1
	}

	return result, col_names
}

func (server *Server) GetPrefixTree() *trees.PrefixTree {
	return server.prefixTree
}

func (server *Server) GetPrefixTreeRootHash() []byte {
	return server.prefixTree.GetHash()
}

func (server *Server) GetStorageCost() int {
	// non-trivial fields: prefix tree, prefixes, commitment tree roots
	totalSize := server.prefixTree.GetSize()
	// for _, prefix := range server.prefixes {
	// 	totalSize += len(prefix)
	// }
	// for _, commitment := range server.commitmentTrees {
	// 	totalSize += len(commitment)
	// }
	fmt.Println("tree node count", server.prefixTree.GetNodeCount())
	return totalSize
}

func (server *Server) AddToGivenSqlTable(table [][]int, names []string, tableName string, db *sql.DB) {
	for i := 0; i < len(table[0]); i++ {
		query := "insert into " + tableName + " values ("
		for j := 0; j < len(table)-1; j++ {
			query = query + strconv.Itoa(table[j][i]) + ", "
		}
		query = query + strconv.Itoa(table[len(table)-1][i]) + ");"

		add_vals, err := db.Query(query)
		if err != nil {
			panic(err.Error())
		}
		add_vals.Close()
	}
}

func (server *Server) CreateAndWriteSqlTable(table [][]int, names []string, tableName string) {
	// assumes user with name "root", no password, database named 'integridb'
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	drop, err := db.Query("DROP TABLE IF EXISTS " + tableName + ";")
	if err != nil {
		panic(err.Error())
	}
	defer drop.Close()

	query := "CREATE TABLE " + tableName + " ("
	for i := 0; i < len(names)-1; i++ {
		query = query + names[i] + " int, "
	}
	query = query + names[len(names)-1] + " int);"

	create, err := db.Query(query)
	if err != nil {
		panic(err.Error())
	}
	defer create.Close()

	server.AddToGivenSqlTable(table, names, tableName, db)
}

func (server *Server) AddToSqlTable(filename string, tableName string) []string {
	table, columnNames := LoadTable(filename)

	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	server.AddToGivenSqlTable(table, columnNames, tableName, db)

	return columnNames
}

func (server *Server) InitializeSqlTable(filename string, tableName string) []string {
	table, columnNames := LoadTable(filename)

	server.CreateAndWriteSqlTable(table, columnNames, tableName)

	return columnNames
}

func (server *Server) addAllUniques(columnName string, tableName string, db *sql.DB, valRange []uint32) []uint32 {
	newUniques := make([]uint32, 0)

	uniquesQuery := "SELECT DISTINCT " + columnName + " FROM " + tableName + " WHERE " + columnName + " >= " + strconv.Itoa(int(valRange[0])) + " AND " + columnName + " < " + strconv.Itoa(int(valRange[1])) + "  ;"
	timeslotResult, err := db.Query(uniquesQuery)
	util.Check(err)
	for timeslotResult.Next() {
		var d int
		err = timeslotResult.Scan(&d)
		if err != nil {
			panic(err.Error())
		}
		newUniques = append(newUniques, uint32(d))
	}

	return newUniques
}

func sortValArray(valArray [][]int, k int) { // sort by the i'th value in the array
	// from https://stackoverflow.com/questions/55360091/golang-sort-slices-of-slice-by-first-element
	sort.Slice(valArray, func(i, j int) bool {
		// edge cases
		if len(valArray[i]) <= k && len(valArray[j]) <= k {
			return false // two insufficiently long slices - so one is not less than other i.e. false
		}
		if len(valArray[i]) <= k || len(valArray[j]) <= k {
			return len(valArray[i]) <= k // insufficiently long slice listed "first"
		}

		// both slices len() > k, so can test this now:
		return valArray[i][k] < valArray[j][k]
	})
}

func (server *Server) getVals(timeVal uint32, miscVals []uint32, db *sql.DB) ([][]int, [][]int, []int, []int) {
	valArray := make([][]int, 0)

	valsQuery := "SELECT " + server.columnNames[server.userRowIdx] + ", " + server.columnNames[server.timeRowIdx] + ", " + server.columnNames[server.dataRowIdx] + ", " + server.columnNames[server.seedRowIdx] + " FROM " + server.tableName + " WHERE " + server.columnNames[server.timeRowIdx] + " = " + strconv.Itoa(int(timeVal))
	numMiscFields := len(server.miscFieldsIdxs)
	for i := 0; i < numMiscFields; i++ {
		valsQuery += " AND " + server.columnNames[server.miscFieldsIdxs[i]] + " = " + strconv.Itoa(int(miscVals[i]))
	}
	valsQuery += ";"

	valsResult, err := db.Query(valsQuery)
	util.Check(err)

	for valsResult.Next() {
		var user, time, val, seed int
		err = valsResult.Scan(&user, &time, &val, &seed)
		if err != nil {
			panic(err.Error())
		}
		valArray = append(valArray, []int{val, seed, user, time})
	}

	sortValArray(valArray, 0)

	vals := make([]int, 0)
	seeds := make([]int, 0)
	users := make([]int, 0)
	times := make([]int, 0)

	for i := 0; i < len(valArray); i++ {
		vals = append(vals, valArray[i][0])
		seeds = append(seeds, valArray[i][1])
		users = append(users, valArray[i][2])
		times = append(times, valArray[i][3])
	}

	valMoments := util.AllValMoments(vals, N_MOMENTS)
	seedMoments := util.AllValMoments(seeds, N_MOMENTS)

	return valMoments, seedMoments, users, times
}

func (server *Server) addDataPointsToPrefixTree(prefix []byte, uniqueMiscVals [][]uint32, uniqueTimeVals []uint32, timeVal uint32, miscVals []uint32, db *sql.DB) {
	// the first part of the prefix is the times
	nUniqueTimes := len(uniqueTimeVals)
	if nUniqueTimes > 0 {
		for i := 0; i < nUniqueTimes; i++ {
			timeBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(timeBytes, uint32(uniqueTimeVals[i]))
			server.addDataPointsToPrefixTree(timeBytes, uniqueMiscVals, make([]uint32, 0), uniqueTimeVals[i], miscVals, db)
		}
		return
	}

	// then the misc columns
	nUniqueCols := len(uniqueMiscVals)
	if nUniqueCols > 0 {
		// we process the misc columns in the order of the slice
		nUniquesInCol := len(uniqueMiscVals[0])
		if nUniquesInCol == 0 {
			fmt.Println("error: empty column!")
		}
		for i := 0; i < nUniquesInCol; i++ {
			colBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(colBytes, uint32(uniqueMiscVals[0][i]))
			newPrefix := append(prefix, colBytes...)
			truncatedMiscVals := uniqueMiscVals[1:]
			newMiscVals := append(miscVals, uniqueMiscVals[0][i])
			server.addDataPointsToPrefixTree(newPrefix, truncatedMiscVals, make([]uint32, 0), timeVal, newMiscVals, db)
		}
		return
	}

	sqlStart := time.Now().UnixNano()
	// finally, create a commitment tree for the values with this unique time/misc vals combo
	// first get the values and seeds
	vals, seeds, users, times := server.getVals(timeVal, miscVals, db)
	server.sqlDuration += time.Now().UnixNano() - sqlStart

	cmtStart := time.Now().UnixNano()
	// then build the commitment tree
	commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
	server.buildCommitmentDuration += time.Now().UnixNano() - cmtStart

	var rootHash [32]byte
	if commitmentTree != nil {
		rootHash = commitmentTree.GetRootHash()
		bitPrefix := util.ConvertBytesToBits(prefix)
		server.prefixTree.PrefixAppend(bitPrefix, rootHash[:])
		server.prefixes = append(server.prefixes, prefix)
		server.commitmentTrees = append(server.commitmentTrees, rootHash[:])
	}
}

func (server *Server) AddToTree(columnNames []string, tableName string, valRange [][]uint32) {

	sqlStart := time.Now().UnixNano()

	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	// take out sql parts
	uniqueMiscs := make([][]uint32, 0)
	for i := 0; i < len(server.miscFieldsIdxs); i++ {
		j := server.miscFieldsIdxs[i]
		uniqueMiscs = append(uniqueMiscs, server.addAllUniques(columnNames[j], tableName, db, valRange[i+1]))
	}

	uniqueTimes := server.addAllUniques(columnNames[server.timeRowIdx], tableName, db, valRange[0])

	server.sqlDuration += time.Now().UnixNano() - sqlStart

	// fmt.Println()
	// fmt.Print("unique miscs: ")
	// fmt.Println(uniqueMiscs)
	// fmt.Print("unique times: ")
	// fmt.Println(uniqueTimes)

	isNotEmpty := len(uniqueTimes) > 0
	for _, uniques := range uniqueMiscs {
		isNotEmpty = isNotEmpty && len(uniques) > 0
	}

	if isNotEmpty {
		server.addDataPointsToPrefixTree(make([]byte, 0), uniqueMiscs, uniqueTimes, 0, make([]uint32, 0), db)
	}
}

func (server *Server) InitializeTree(columnNames []string, tableName string, userRowIdx, timeRowIdx, dataRowIdx, seedRowIdx uint32, miscFieldsIdxs []uint32) {
	server.prefixTree = trees.NewPrefixTree()
	server.prefixes = make([][]byte, 0)
	server.commitmentTrees = make([][]byte, 0)

	server.columnNames = columnNames
	server.tableName = tableName
	server.userRowIdx = userRowIdx
	server.timeRowIdx = timeRowIdx
	server.dataRowIdx = dataRowIdx
	server.seedRowIdx = seedRowIdx
	server.miscFieldsIdxs = miscFieldsIdxs

	allVals := make([][]uint32, 1+len(miscFieldsIdxs))
	for i := range allVals {
		allVals[i] = []uint32{0, math.MaxInt32}
	}

	server.AddToTree(columnNames, tableName, allVals)
}

func (server *Server) RequestPrefixMembershipProof(prefix []byte) []byte {
	server.prefixReceiveRequestTime = time.Now().UnixNano()
	bitPrefix := util.ConvertBytesToBits(prefix)
	proof, leafValue := server.prefixTree.GenerateMembershipProof(bitPrefix)
	output := NewPrefixMembershipProofOutput(proof, leafValue)
	outputBytes, err := json.Marshal(output)
	util.Check(err)
	server.prefixSendResponseTime = time.Now().UnixNano()
	return outputBytes
}

// func (server *Server) RequestPrefixNonMembershipProof(vals []uint32) (proof *trees.NonMembershipProof) {
// 	prefix := server.valsToPrefix(vals)
// 	bitPrefix := util.ConvertBytesToBits(prefix)
// 	return server.prefixTree.GenerateNonMembershipProof(bitPrefix)
// }

func (server *Server) RequestCommitmentTreeMembershipProof(inputBytes []byte) []byte {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	var input *CommitmentMembershipProofInput
	json.Unmarshal(inputBytes, &input)
	prefix, val, uid := input.Parse()

	// sql part for lookUp
	sqlStart := time.Now().UnixNano()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	colvals := util.PrefixToVals(prefix)
	vals, seeds, users, times := server.getVals(colvals[0], colvals[1:], db)

	server.sqlDuration += time.Now().UnixNano() - sqlStart

	cmtStart := time.Now().UnixNano()

	commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
	output := commitmentTree.GenerateMembershipProof(int(val), int(uid), false)
	outputBytes, err := json.Marshal(NewCommitmentMembershipProofOutput(output))
	util.Check(err)

	server.buildCommitmentDuration += time.Now().UnixNano() - cmtStart
	server.querySendResponseTime = time.Now().UnixNano()
	return outputBytes
}

// func (server *Server) RequestCommitmentTreeMembershipProof(inputBytes []byte) []byte {
// 	var input *CommitmentMembershipProofInput
// 	json.Unmarshal(inputBytes, &input)
// 	prefix, val, uid := input.Parse()
// 	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
// 	util.Check(err)
// 	defer db.Close()

// 	colvals := util.PrefixToVals(prefix)
// 	vals, seeds, users, times := server.getVals(colvals[0], colvals[1:], db)

// 	commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
// 	output := commitmentTree.GenerateMembershipProof(int(val), int(uid))
// 	outputBytes, err := json.Marshal(output)
// 	util.Check(err)
// 	return outputBytes
// }

func (server *Server) RequestCommitmentsAndProofs(prefix []byte) []byte {
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	colvals := util.PrefixToVals(prefix)
	vals, seeds, users, times := server.getVals(colvals[0], colvals[1:], db)

	n := len(vals)

	H, err0 := p256.MapToGroup(trees.COMMIT_SEEDH)
	util.Check(err0)

	commitments := make([][]*p256.P256, 0)
	idHashes := make([][32]byte, 0)
	proofs := make([]*bulletproofs.ProofBPRP, 0)
	params, err := bulletproofs.SetupGeneric(0, BP_MAX)
	util.Check(err)

	for i := 0; i < n; i++ {
		momentCommitments := make([]*p256.P256, 0)
		for j := 0; j < len(vals[0]); j++ {
			bigVal := big.NewInt(int64(vals[i][j]))
			bigSeed := big.NewInt(int64(seeds[i][j]))
			C, err1 := p256.CommitG1(bigVal, bigSeed, H)
			util.Check(err1)
			momentCommitments = append(momentCommitments, C)
		}
		idHashes = append(idHashes, util.IdHash(users[i], times[i]))

		if i < n-1 {
			bigValDiff := big.NewInt(int64(vals[i+1][0] - vals[i][0]))
			bigSeedDiff := big.NewInt(int64(seeds[i+1][0] - seeds[i][0]))

			proof, err := bulletproofs.ProveGeneric(bigValDiff, params, bigSeedDiff)
			util.Check(err)
			proofs = append(proofs, &proof)
		}

		commitments = append(commitments, momentCommitments)
	}

	output := NewCommitmentsAndProofs(commitments, idHashes, proofs)
	outputBytes, err := json.Marshal(output)
	util.Check(err)
	return outputBytes
}

func (server *Server) RequestPrefix(valsBytes []byte) []byte {
	vals := util.BytesToInts(valsBytes)
	m := len(vals)
	// we can re-use existing functions
	valRange := make([][]uint32, m)
	for i := 0; i < m; i++ {
		valRange[i] = []uint32{vals[i], vals[i] + 1}
	}
	valInput := NewPrefixesInput(valRange)
	valBytes, err := json.Marshal(valInput)
	util.Check(err)
	prefixOutputBytes := server.RequestPrefixes(valBytes)
	var output *PrefixesOutput
	json.Unmarshal(prefixOutputBytes, &output)
	if len(output.ValidPrefixes) == 0 {
		return nil
	}
	return output.ValidPrefixes[0].Slice
}

func (server *Server) RequestPrefixes(valBytes []byte) []byte {
	server.prefixReceiveRequestTime = time.Now().UnixNano()
	var input *PrefixesInput
	json.Unmarshal(valBytes, &input)
	valRange := input.Parse()
	validPrefixes := make([][]byte, 0)
	validTreeRoots := make([][]byte, 0)
	n := len(server.prefixes)
	m := len(valRange)
	for i := 0; i < n; i++ {
		prefix := server.prefixes[i]
		vals := util.PrefixToVals(prefix)
		isValid := true
		for j := 0; j < m; j++ {
			isValid = isValid && vals[j] >= valRange[j][0]
			isValid = isValid && vals[j] < valRange[j][1]
		}
		if isValid {
			validPrefixes = append(validPrefixes, prefix)
			validTreeRoots = append(validTreeRoots, server.commitmentTrees[i])
		}
	}
	setCompletenessProof := server.prefixTree.GenerateSetCompletenessProof(valRange, 4)
	serializedProof, err := setCompletenessProof.Serialize()
	util.Check(err)
	output := NewPrefixesOutput(validPrefixes, validTreeRoots, serializedProof)
	outputBytes, err := json.Marshal(output)
	util.Check(err)
	server.prefixSendResponseTime = time.Now().UnixNano()
	return outputBytes
}

func (server *Server) RequestSumAndCounts(queryPrefixes [][]byte) (int, int, [][][]byte, []int, [][]byte) {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	totValSum := 0
	totSeedSum := 0
	commitments := make([][][]byte, 0)
	counts := make([]int, 0)
	hashes := make([][]byte, 0)

	// iterate over the prefixes and determine the hash without the commitments in each commitment tree root
	n := len(queryPrefixes)
	// fmt.Println("---")
	for i := 0; i < n; i++ {
		hash := make([]byte, 0)
		prefix := queryPrefixes[i]
		colvals := util.PrefixToVals(prefix)
		vals, seeds, users, times := server.getVals(colvals[0], colvals[1:], db)
		commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
		if commitmentTree != nil {
			hash = commitmentTree.GetRootChildHash()
			commitments = append(commitments, commitmentTree.GetRootCommitments())
		} else {
			commitments = append(commitments, nil)
		}

		valSum := 0
		seedSum := 0

		m := len(vals)
		for j := 0; j < m; j++ {
			valSum += vals[j][0]
			seedSum += seeds[j][0]
		}

		totValSum += valSum
		totSeedSum += seedSum
		counts = append(counts, m)
		hashes = append(hashes, hash)
	}

	server.querySendResponseTime = time.Now().UnixNano()
	return totValSum, totSeedSum, commitments, counts, hashes
}

func (server *Server) RequestMinAndProofs(queryPrefixInputBytes []byte) (int, int, []byte, [][]byte, [][]byte) {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	var queryPrefixInput *PrefixSet
	json.Unmarshal(queryPrefixInputBytes, &queryPrefixInput)
	queryPrefixes := queryPrefixInput.Parse()
	n := len(queryPrefixes)
	vals := make([][][]int, n)
	seeds := make([][][]int, n)
	users := make([][]int, n)
	times := make([][]int, n)
	rangeProofs := make([]*bulletproofs.ProofBPRP, n)
	inclusionProofs := make([]*trees.CommitMembershipProof, n)

	minVal := math.MaxInt32
	argminVal := -1
	localMins := make([]int, n)

	for i := 0; i < n; i++ {
		prefix := queryPrefixes[i]
		colvals := util.PrefixToVals(prefix)
		vals[i], seeds[i], users[i], times[i] = server.getVals(colvals[0], colvals[1:], db)

		// vals are sorted, so element 0 is the smallest
		localMins[i] = vals[i][0][0]
		if localMins[i] < minVal {
			minVal = localMins[i]
			argminVal = i
		}
	}

	params, err := bulletproofs.SetupGeneric(int64(minVal), BP_MAX)
	util.Check(err)
	var proofMin bulletproofs.ProofBPRP

	fmt.Print("building proofs: ")
	for i := 0; i < n; i++ {
		fmt.Print(strconv.Itoa(i) + "/" + strconv.Itoa(n) + ", ")
		commitmentTree := trees.BuildMerkleCommitmentTree(vals[i], seeds[i], users[i], times[i])
		inclusionProof := commitmentTree.GenerateMembershipProof(localMins[i], -1, true)
		inclusionProofs[i] = inclusionProof
		proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][0][0])), params, big.NewInt(int64(seeds[i][0][0])))
		util.Check(err)
		rangeProofs[i] = &proof

		if i == argminVal {
			paramsMin, err := bulletproofs.SetupGeneric(int64(minVal), int64(minVal+1))
			util.Check(err)
			proofMin, err = bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][0][0])), paramsMin, big.NewInt(int64(seeds[i][0][0])))
			util.Check(err)
		}
	}
	fmt.Println()

	proofMinBytes, err := json.Marshal(proofMin)
	util.Check(err)
	rangeProofsBytes := make([][]byte, len(rangeProofs))
	for i, proof := range rangeProofs {
		rangeProofsBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}
	inclusionProofsBytes := make([][]byte, len(inclusionProofs))
	for i, proof := range inclusionProofs {
		inclusionProofsBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}

	server.querySendResponseTime = time.Now().UnixNano()
	return minVal, argminVal, proofMinBytes, rangeProofsBytes, inclusionProofsBytes
}

func (server *Server) RequestMaxAndProofs(queryPrefixes [][]byte) (int, int, []byte, [][]byte, [][]byte) {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	n := len(queryPrefixes)
	vals := make([][][]int, n)
	seeds := make([][][]int, n)
	users := make([][]int, n)
	times := make([][]int, n)
	rangeProofs := make([]*bulletproofs.ProofBPRP, n)
	inclusionProofs := make([]*trees.CommitMembershipProof, n)

	maxVal := math.MinInt32
	argmaxVal := -1
	localMaxs := make([]int, n)

	for i := 0; i < n; i++ {
		prefix := queryPrefixes[i]
		colvals := util.PrefixToVals(prefix)
		vals[i], seeds[i], users[i], times[i] = server.getVals(colvals[0], colvals[1:], db)

		// vals are sorted, so element at end of array is the largest

		localMaxs[i] = vals[i][len(vals[i])-1][0]
		if localMaxs[i] > maxVal {
			maxVal = localMaxs[i]
			argmaxVal = i
		}
	}

	fmt.Print("max: ")
	fmt.Println(maxVal)

	params, err := bulletproofs.SetupGeneric(0, int64(maxVal+1))
	util.Check(err)
	var proofMax bulletproofs.ProofBPRP

	fmt.Print("building proofs: ")
	for i := 0; i < n; i++ {
		fmt.Print(strconv.Itoa(i) + "/" + strconv.Itoa(n) + ", ")
		commitmentTree := trees.BuildMerkleCommitmentTree(vals[i], seeds[i], users[i], times[i])
		inclusionProof := commitmentTree.GenerateMembershipProof(localMaxs[i], -1, false)
		inclusionProofs[i] = inclusionProof
		proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][len(vals[i])-1][0])), params, big.NewInt(int64(seeds[i][len(vals[i])-1][0])))
		util.Check(err)
		rangeProofs[i] = &proof

		if i == argmaxVal {
			paramsMax, err := bulletproofs.SetupGeneric(int64(maxVal), int64(maxVal+1))
			util.Check(err)
			// fmt.Println(strconv.Itoa(vals[i][len(vals[i])-1]) + "inside [" + strconv.Itoa(maxVal) + ", " + strconv.Itoa(maxVal+1) + ")?")
			proofMax, err = bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][len(vals[i])-1][0])), paramsMax, big.NewInt(int64(seeds[i][len(vals[i])-1][0])))
			util.Check(err)
		}
	}
	fmt.Println()

	proofMaxBytes, err := json.Marshal(proofMax)
	util.Check(err)
	rangeProofsBytes := make([][]byte, len(rangeProofs))
	for i, proof := range rangeProofs {
		rangeProofsBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}
	inclusionProofBytes := make([][]byte, len(inclusionProofs))
	for i, proof := range inclusionProofs {
		inclusionProofBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}

	server.querySendResponseTime = time.Now().UnixNano()
	return maxVal, argmaxVal, proofMaxBytes, rangeProofsBytes, inclusionProofBytes
}

func (server *Server) RequestQuantileAndProofs(queryPrefixes [][]byte, k int, q int) (int, int, [][]byte, [][]byte, [][]byte, [][]byte) {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/integridb")
	util.Check(err)
	defer db.Close()

	n := len(queryPrefixes)
	vals := make([][][]int, n)
	seeds := make([][][]int, n)
	users := make([][]int, n)
	times := make([][]int, n)
	valSeedIdxPairs := make([][]int, 0)

	// first determine the quantile
	for i := 0; i < n; i++ {
		prefix := queryPrefixes[i]
		colvals := util.PrefixToVals(prefix)
		vals[i], seeds[i], users[i], times[i] = server.getVals(colvals[0], colvals[1:], db)
		m := len(vals[i])
		for j := 0; j < m; j++ {
			valSeedIdxPairs = append(valSeedIdxPairs, []int{vals[i][j][0], seeds[i][j][0]})
		}
	}

	sortValArray(valSeedIdxPairs, 0)
	var quantileFloat float64

	// based on the python implementation:
	mm := len(valSeedIdxPairs)
	if mm < 1 {
		fmt.Println("warning: no entries found that match the search criteria")
		return -1, -1, nil, nil, nil, nil
	}
	kk := float64(mm-1) * float64(k) / float64(q)
	ff := math.Floor(kk)
	cc := math.Ceil(kk)
	if ff == cc {
		quantileFloat = float64(valSeedIdxPairs[int(kk)][0])
	} else {
		d0 := float64(valSeedIdxPairs[int(ff)][0]) * (cc - kk)
		d1 := float64(valSeedIdxPairs[int(cc)][0]) * (kk - ff)
		quantileFloat = d0 + d1
	}
	// round down, as we can only do range proofs on integers:
	quantile := int(math.Floor(quantileFloat))
	// determine how many duplicates there are of the quantile:
	nCount := 0
	for i := int(ff); i >= 0; i-- { // no need to search the full array - can start from ff and cc and search down/up
		if valSeedIdxPairs[i][0] == quantile {
			nCount++
		} else {
			break
		}
	}
	for i := int(ff + 1); i < len(valSeedIdxPairs); i++ {
		if valSeedIdxPairs[i][0] == quantile {
			nCount++
		} else {
			break
		}
	}

	inclusionProofsLeft := make([]*trees.CommitMembershipProof, n)
	rangeProofsLeft := make([]*bulletproofs.ProofBPRP, n)
	inclusionProofsRight := make([]*trees.CommitMembershipProof, n)
	rangeProofsRight := make([]*bulletproofs.ProofBPRP, n)

	paramsQuantileLeft, err := bulletproofs.SetupGeneric(0, int64(quantile+1))
	util.Check(err)
	paramsQuantileRight, err := bulletproofs.SetupGeneric(int64(quantile), BP_MAX)
	util.Check(err)

	fmt.Println("total # nodes: " + strconv.Itoa(mm))
	fmt.Print("building proofs: ")
	for i := 0; i < n; i++ {
		fmt.Print(strconv.Itoa(i) + "/" + strconv.Itoa(n) + ", ")
		m := len(vals[i])
		highestVal := -1
		highestValIdx := -1
		lowestVal := -1
		lowestValIdx := -1
		// find as many nodes as possible whose value is at most equal to the quantile
		for j := 0; j < m; j++ {
			if vals[i][j][0] <= quantile {
				highestVal = vals[i][j][0]
				highestValIdx = j
			}
		}
		// find as many nodes as possible whose value is at least equal to the quantile
		for j := m - 1; j >= 0; j-- {
			if vals[i][j][0] >= quantile {
				lowestVal = vals[i][j][0]
				lowestValIdx = j
			}
		}
		// build the Merkle tree
		commitmentTree := trees.BuildMerkleCommitmentTree(vals[i], seeds[i], users[i], times[i])
		// create the proofs
		if highestValIdx > -1 {
			proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][highestValIdx][0])), paramsQuantileLeft, big.NewInt(int64(seeds[i][highestValIdx][0])))
			util.Check(err)
			rangeProofsLeft[i] = &proof
			inclusionProof := commitmentTree.GenerateMembershipProof(highestVal, -1, false)
			inclusionProofsLeft[i] = inclusionProof
		}
		if lowestValIdx > -1 {
			proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[i][lowestValIdx][0])), paramsQuantileRight, big.NewInt(int64(seeds[i][lowestValIdx][0])))
			util.Check(err)
			rangeProofsRight[i] = &proof
			inclusionProof := commitmentTree.GenerateMembershipProof(lowestVal, -1, true)
			inclusionProofsRight[i] = inclusionProof
		}
	}
	fmt.Println()

	inclusionProofsLeftBytes := make([][]byte, len(inclusionProofsLeft))
	for i, proof := range inclusionProofsLeft {
		inclusionProofsLeftBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}
	rangeProofsLeftBytes := make([][]byte, len(rangeProofsLeft))
	for i, proof := range rangeProofsLeft {
		rangeProofsLeftBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}
	inclusionProofsRightBytes := make([][]byte, len(inclusionProofsRight))
	for i, proof := range inclusionProofsRight {
		inclusionProofsRightBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}
	rangeProofsRightBytes := make([][]byte, len(rangeProofsRight))
	for i, proof := range rangeProofsRight {
		rangeProofsRightBytes[i], err = json.Marshal(proof)
		util.Check(err)
	}

	server.querySendResponseTime = time.Now().UnixNano()
	return quantile, nCount, inclusionProofsLeftBytes, rangeProofsLeftBytes, inclusionProofsRightBytes, rangeProofsRightBytes
}
