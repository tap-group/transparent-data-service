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
	"sync"
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
const MAX_THREADS = 100
const MAX_ENTRIES_PER_INSERT = 100

type Server struct {
	columnNames []string
	tableName   string
	userRowIdx,
	timeRowIdx,
	dataRowIdx,
	seedRowIdx uint32
	typeFieldsIdxs []uint32

	prefixTree      *trees.PrefixTree
	prefixes        [][]byte
	commitmentTrees [][]byte

	// the following variables are for benchmarking (intended for single-thread exec):

	prefixReceiveRequestTime int64
	prefixSendResponseTime   int64
	queryReceiveRequestTime  int64
	querySendResponseTime    int64

	buildTreeTime   int64
	buildProofsTime int64

	sqlDuration             int64
	buildCommitmentDuration int64
}

type SumTreeData struct {
	prefix   []byte
	timeVal  uint32
	typeVals []uint32
}

func (server *Server) GetSqlDuration() int64 {
	return server.sqlDuration
}

func (server *Server) GetBuildTreeTime() int64 {
	return server.buildTreeTime
}

func (server *Server) GetBuildProofsTime() int64 {
	return server.buildProofsTime
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

// ResetTimestamps resets all the time counters that are used for tracing

func (server *Server) ResetTimestamps() {
	server.prefixReceiveRequestTime = time.Now().UnixNano()
	server.prefixSendResponseTime = time.Now().UnixNano()
	server.queryReceiveRequestTime = time.Now().UnixNano()
	server.querySendResponseTime = time.Now().UnixNano()
}

// LoadTable loads a table from a file and returns the values and column names
// It takes the file's path as a string as input and returns the values and column names as a 2-dimensional int slice and a string slice
// In the current implementation, LoadTable is used to load tables before inserting them into the MySQL database

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

// GetPrefixTree returns the prefix tree

func (server *Server) GetPrefixTree() *trees.PrefixTree {
	return server.prefixTree
}

// GetPrefixTreeRootHash returns the hash of the prefix tree root

func (server *Server) GetPrefixTreeRootHash() []byte {
	return server.prefixTree.GetHash()
}

// GetStorageCost returns the byte size of the prefix tree

func (server *Server) GetStorageCost() int {
	totalSize := server.prefixTree.GetSize()
	return totalSize
}

// AddToGivenSqlTable inserts a set of values into the SQL database

func (server *Server) AddToGivenSqlTable(values [][]int, columnnNames []string, tableName string, db *sql.DB) {
	var wg sync.WaitGroup
	// we divide the entries into threads, and then each thread into batches
	// the batch is set to the square root of the the number of entries per thread, with a maximum of 100 entries per batch
	numEntriesPerThread := int(math.Ceil(float64(len(values[0])) / float64(MAX_THREADS)))
	numEntriesPerBatch := int(math.Ceil(math.Sqrt(float64(numEntriesPerThread))))
	if numEntriesPerBatch > MAX_ENTRIES_PER_INSERT {
		numEntriesPerBatch = int(math.Ceil(float64(numEntriesPerThread) / float64(MAX_ENTRIES_PER_INSERT)))
	}
	numBatchesPerThread := int(math.Ceil(float64(numEntriesPerThread) / float64(numEntriesPerBatch)))
	totalAdded := make([]int, MAX_THREADS)
	for i := range totalAdded {
		totalAdded[i] = 0
	}
	for z := 0; z < MAX_THREADS; z++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			for c := 0; c < numBatchesPerThread; c++ {
				// k is the index of the thread, c is the entry of the batch
				mmin := k*numEntriesPerThread + c*numEntriesPerBatch
				mmax := k*numEntriesPerThread + (c+1)*numEntriesPerBatch
				if mmax > (k+1)*numEntriesPerThread {
					mmax = (k + 1) * numEntriesPerThread
				}
				if mmax > len(values[0]) {
					mmax = len(values[0])
				}
				if mmax-mmin > 0 {
					query := "insert into " + tableName + " values "
					for i := mmin; i < mmax; i++ {
						query += "("
						for j := 0; j < len(values); j++ {
							query += strconv.Itoa(values[j][i])
							if j < len(values)-1 {
								query += ","
							}
						}
						if i < mmax-1 {
							query += "),"
						} else {
							query += ");"
						}
						totalAdded[k]++
					}

					add_vals, err := db.Query(query)
					if err != nil {
						panic(err.Error())
					}
					add_vals.Close()
				}
			}
		}(z)
	}
	wg.Wait()
}

// GetOpenedDatabase returns an open connection to the MySQL database
// It assumes that the database can be accessed by a user with name "root" and no password, and that there is a database named 'tap'

func (server *Server) GetOpenedDatabase() (db *sql.DB) {
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/tap")
	util.Check(err)
	return db
}

// CreateAndWriteSqlTable creates a MySQL table and insers the given values and column names

func (server *Server) CreateAndWriteSqlTable(values [][]int, columnNames []string, tableName string) {
	db := server.GetOpenedDatabase()
	defer db.Close()

	drop, err := db.Query("DROP TABLE IF EXISTS " + tableName + ";")
	if err != nil {
		panic(err.Error())
	}
	defer drop.Close()

	query := "CREATE TABLE " + tableName + " ("
	for i := 0; i < len(columnNames)-1; i++ {
		query = query + columnNames[i] + " int, "
	}
	query = query + columnNames[len(columnNames)-1] + " int);"

	create, err := db.Query(query)
	if err != nil {
		panic(err.Error())
	}
	defer create.Close()

	server.AddToGivenSqlTable(values, columnNames, tableName, db)
}

func (server *Server) AddToSqlTable(filename string, tableName string) []string {
	table, columnNames := LoadTable(filename)

	db := server.GetOpenedDatabase()
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

// sortValArray sorts a 2-dimensional slice by the i'th value in the slice

func sortValArray(valArray [][]int, k int) {
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

// getVals performs a MySQL database query to obtain the value corresponding to the given time and type values

func (server *Server) getVals(timeVal uint32, typeVals []uint32, db *sql.DB) ([][]int, [][]int, []int, []int) {
	valArray := make([][]int, 0)

	valsQuery := "SELECT " + server.columnNames[server.userRowIdx] + ", " + server.columnNames[server.timeRowIdx] + ", " + server.columnNames[server.dataRowIdx] + ", " + server.columnNames[server.seedRowIdx] + " FROM " + server.tableName + " WHERE " + server.columnNames[server.timeRowIdx] + " = " + strconv.Itoa(int(timeVal))
	numTypeFields := len(server.typeFieldsIdxs)
	for i := 0; i < numTypeFields; i++ {
		valsQuery += " AND " + server.columnNames[server.typeFieldsIdxs[i]] + " = " + strconv.Itoa(int(typeVals[i]))
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

func (server *Server) determinePrefixes(prefix []byte, uniqueTypeVals [][]uint32, uniqueTimeVals []uint32, timeVal uint32, typeVals []uint32, result *[]SumTreeData) {
	// the first part of the prefix is the times
	nUniqueTimes := len(uniqueTimeVals)
	if nUniqueTimes > 0 {
		for i := 0; i < nUniqueTimes; i++ {
			timeBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(timeBytes, uint32(uniqueTimeVals[i]))
			server.determinePrefixes(timeBytes, uniqueTypeVals, make([]uint32, 0), uniqueTimeVals[i], typeVals, result)
		}
		return
	}

	// then the type columns
	nUniqueCols := len(uniqueTypeVals)
	if nUniqueCols > 0 {
		// we process the type columns in the order of the slice
		nUniquesInCol := len(uniqueTypeVals[0])
		if nUniquesInCol == 0 {
			fmt.Println("error: empty column!")
		}
		for i := 0; i < nUniquesInCol; i++ {
			colBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(colBytes, uint32(uniqueTypeVals[0][i]))
			newPrefix := append(prefix, colBytes...)
			truncatedTypeVals := uniqueTypeVals[1:]
			newTypeVals := append(typeVals, uniqueTypeVals[0][i])
			server.determinePrefixes(newPrefix, truncatedTypeVals, make([]uint32, 0), timeVal, newTypeVals, result)
		}
		return
	}

	typeValCopy := make([]uint32, len(typeVals))
	copy(typeValCopy, typeVals)
	*result = append(*result, SumTreeData{prefix: prefix, timeVal: timeVal, typeVals: typeValCopy})
}

func (server *Server) AddToTree(columnNames []string, tableName string, valRange [][]uint32) {
	sqlStart := time.Now().UnixNano()

	db := server.GetOpenedDatabase()
	defer db.Close()

	// take out sql parts
	uniqueTypes := make([][]uint32, 0)
	for i := 0; i < len(server.typeFieldsIdxs); i++ {
		j := server.typeFieldsIdxs[i]
		uniqueTypes = append(uniqueTypes, server.addAllUniques(columnNames[j], tableName, db, valRange[i+1]))
	}

	uniqueTimes := server.addAllUniques(columnNames[server.timeRowIdx], tableName, db, valRange[0])

	server.sqlDuration += time.Now().UnixNano() - sqlStart

	isNotEmpty := len(uniqueTimes) > 0
	for _, uniques := range uniqueTypes {
		isNotEmpty = isNotEmpty && len(uniques) > 0
	}

	sumTreeData := make([]SumTreeData, 0)
	server.determinePrefixes(make([]byte, 0), uniqueTypes, uniqueTimes, 0, make([]uint32, 0), &sumTreeData)

	l := int(math.Ceil(float64(len(sumTreeData)) / float64(MAX_THREADS)))
	n := int(math.Ceil(float64(len(sumTreeData)) / float64(l)))

	newPrefixes := make([][]byte, 0)
	newTrees := make([][]byte, 0)
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	if isNotEmpty {
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				mmin := k * l
				mmax := (k + 1) * l
				if mmax > len(sumTreeData) {
					mmax = len(sumTreeData)
				}
				if mmax-mmin > 0 {
					for j := mmin; j < mmax; j++ {
						treeData := sumTreeData[j]
						sqlStart := time.Now().UnixNano()
						// finally, create a commitment tree for the values with this unique time/type vals combo
						// first get the values and seeds
						vals, seeds, users, times := server.getVals(treeData.timeVal, treeData.typeVals, db)
						server.sqlDuration += time.Now().UnixNano() - sqlStart

						cmtStart := time.Now().UnixNano()
						// then build the commitment tree

						commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
						server.buildCommitmentDuration += time.Now().UnixNano() - cmtStart

						var rootHash [32]byte
						if commitmentTree != nil {
							rootHash = commitmentTree.GetRootHash()
							mu.Lock()
							server.prefixes = append(server.prefixes, treeData.prefix)
							server.commitmentTrees = append(server.commitmentTrees, rootHash[:])
							newPrefixes = append(newPrefixes, treeData.prefix)
							newTrees = append(newTrees, rootHash[:])
							mu.Unlock()
						}
					}
				}
			}(i)
		}
	}

	wg.Wait()

	for i := range newPrefixes {
		bitPrefix := util.ConvertBytesToBits(newPrefixes[i])
		server.prefixTree.PrefixAppend(bitPrefix, newTrees[i])
	}
}

func (server *Server) InitializeTree(columnNames []string, tableName string, userRowIdx, timeRowIdx, dataRowIdx, seedRowIdx uint32, typeFieldsIdxs []uint32) {
	server.prefixTree = trees.NewPrefixTree()
	server.prefixes = make([][]byte, 0)
	server.commitmentTrees = make([][]byte, 0)

	server.columnNames = columnNames
	server.tableName = tableName
	server.userRowIdx = userRowIdx
	server.timeRowIdx = timeRowIdx
	server.dataRowIdx = dataRowIdx
	server.seedRowIdx = seedRowIdx
	server.typeFieldsIdxs = typeFieldsIdxs

	allVals := make([][]uint32, 1+len(typeFieldsIdxs))
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

func (server *Server) RequestCommitmentTreeMembershipProof(inputBytes []byte) []byte {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	var input *CommitmentMembershipProofInput
	json.Unmarshal(inputBytes, &input)
	prefix, val, uid := input.Parse()

	// sql part for lookUp
	sqlStart := time.Now().UnixNano()
	db := server.GetOpenedDatabase()
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

func (server *Server) RequestCommitmentsAndProofs(prefix []byte) []byte {
	db := server.GetOpenedDatabase()
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
	db := server.GetOpenedDatabase()
	defer db.Close()

	n := len(queryPrefixes)

	totValSum := 0
	totSeedSum := 0
	commitments := make([][][]byte, n)
	counts := make([]int, n)
	hashes := make([][]byte, n)

	// iterate over the prefixes and determine the hash without the commitments in each commitment tree root
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	startTime := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		wg.Add(1)
		hash := make([]byte, 0)
		prefix := queryPrefixes[i]
		colvals := util.PrefixToVals(prefix)
		vals, seeds, users, times := server.getVals(colvals[0], colvals[1:], db)
		go func(k int) {
			defer wg.Done()
			commitmentTree := trees.BuildMerkleCommitmentTree(vals, seeds, users, times)
			if commitmentTree != nil {
				hash = commitmentTree.GetRootChildHash()
				commitments[k] = commitmentTree.GetRootCommitments()
			} else {
				commitments[k] = nil
			}

			valSum := 0
			seedSum := 0

			m := len(vals)
			for j := 0; j < m; j++ {
				valSum += vals[j][0]
				seedSum += seeds[j][0]
			}

			mu.Lock()
			totValSum += valSum
			totSeedSum += seedSum
			mu.Unlock()

			counts[k] = m
			hashes[k] = hash

		}(i)
	}

	wg.Wait()
	server.buildTreeTime = time.Now().UnixNano() - startTime

	server.querySendResponseTime = time.Now().UnixNano()
	return totValSum, totSeedSum, commitments, counts, hashes
}

func (server *Server) RequestMinAndProofs(queryPrefixInputBytes []byte) (int, int, []byte, [][]byte, [][]byte) {
	server.queryReceiveRequestTime = time.Now().UnixNano()
	db := server.GetOpenedDatabase()
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

	fmt.Print("min: ")
	fmt.Println(minVal)

	params, err := bulletproofs.SetupGeneric(int64(minVal), BP_MAX)
	util.Check(err)
	var proofMin bulletproofs.ProofBPRP
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	server.buildTreeTime = 0
	server.buildProofsTime = 0

	fmt.Println("building proofs...")
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			startBuild := time.Now().UnixNano()
			commitmentTree := trees.BuildMerkleCommitmentTree(vals[k], seeds[k], users[k], times[k])
			buildTime := time.Now().UnixNano() - startBuild
			mu.Lock()
			server.buildTreeTime += buildTime
			mu.Unlock()
			startBuild = time.Now().UnixNano()
			inclusionProof := commitmentTree.GenerateMembershipProof(localMins[k], -1, true)
			inclusionProofs[k] = inclusionProof
			proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][0][0])), params, big.NewInt(int64(seeds[k][0][0])))
			util.Check(err)
			buildTime = time.Now().UnixNano() - startBuild
			mu.Lock()
			server.buildProofsTime += buildTime
			mu.Unlock()
			rangeProofs[k] = &proof

			if k == argminVal {
				paramsMin, err := bulletproofs.SetupGeneric(int64(minVal), int64(minVal+1))
				util.Check(err)
				proofMin, err = bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][0][0])), paramsMin, big.NewInt(int64(seeds[k][0][0])))
				util.Check(err)
			}
		}(i)
	}
	wg.Wait()

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
	db := server.GetOpenedDatabase()
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
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	server.buildTreeTime = 0
	server.buildProofsTime = 0

	fmt.Println("building proofs...")
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			startBuild := time.Now().UnixNano()
			commitmentTree := trees.BuildMerkleCommitmentTree(vals[k], seeds[k], users[k], times[k])
			buildTime := time.Now().UnixNano() - startBuild
			mu.Lock()
			server.buildTreeTime += buildTime
			mu.Unlock()

			startBuild = time.Now().UnixNano()
			inclusionProof := commitmentTree.GenerateMembershipProof(localMaxs[k], -1, false)
			inclusionProofs[k] = inclusionProof
			proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][len(vals[k])-1][0])), params, big.NewInt(int64(seeds[k][len(vals[k])-1][0])))
			util.Check(err)
			buildTime = time.Now().UnixNano() - startBuild
			mu.Lock()
			server.buildProofsTime += buildTime
			mu.Unlock()
			rangeProofs[k] = &proof

			if k == argmaxVal {
				paramsMax, err := bulletproofs.SetupGeneric(int64(maxVal), int64(maxVal+1))
				util.Check(err)
				proofMax, err = bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][len(vals[k])-1][0])), paramsMax, big.NewInt(int64(seeds[k][len(vals[k])-1][0])))
				util.Check(err)
			}
		}(i)
	}
	wg.Wait()

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
	db := server.GetOpenedDatabase()
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
	fmt.Print("quantile: ")
	fmt.Println(quantile)
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

	var wg sync.WaitGroup
	mu := sync.Mutex{}
	server.buildTreeTime = 0
	server.buildProofsTime = 0

	fmt.Println("total # leaves: " + strconv.Itoa(mm))
	fmt.Println("building proofs...")
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			m := len(vals[k])
			highestVal := -1
			highestValIdx := -1
			lowestVal := -1
			lowestValIdx := -1
			// find as many leaves as possible whose value is at most equal to the quantile
			for j := 0; j < m; j++ {
				if vals[k][j][0] <= quantile {
					highestVal = vals[k][j][0]
					highestValIdx = j
				}
			}
			// find as many leaves as possible whose value is at least equal to the quantile
			for j := m - 1; j >= 0; j-- {
				if vals[k][j][0] >= quantile {
					lowestVal = vals[k][j][0]
					lowestValIdx = j
				}
			}
			// build the Merkle tree
			startBuild := time.Now().UnixNano()
			commitmentTree := trees.BuildMerkleCommitmentTree(vals[k], seeds[k], users[k], times[k])
			buildTime := time.Now().UnixNano() - startBuild
			mu.Lock()
			server.buildTreeTime += buildTime
			mu.Unlock()

			// create the proofs
			if highestValIdx > -1 {
				startBuild = time.Now().UnixNano()
				proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][highestValIdx][0])), paramsQuantileLeft, big.NewInt(int64(seeds[k][highestValIdx][0])))
				util.Check(err)
				rangeProofsLeft[k] = &proof
				inclusionProof := commitmentTree.GenerateMembershipProof(highestVal, -1, false)
				inclusionProofsLeft[k] = inclusionProof
				buildTime = time.Now().UnixNano() - startBuild
				mu.Lock()
				server.buildProofsTime += buildTime
				mu.Unlock()
			}
			if lowestValIdx > -1 {
				startBuild = time.Now().UnixNano()
				proof, err := bulletproofs.ProveGeneric(big.NewInt(int64(vals[k][lowestValIdx][0])), paramsQuantileRight, big.NewInt(int64(seeds[k][lowestValIdx][0])))
				util.Check(err)
				rangeProofsRight[k] = &proof
				inclusionProof := commitmentTree.GenerateMembershipProof(lowestVal, -1, true)
				inclusionProofsRight[k] = inclusionProof
				buildTime = time.Now().UnixNano() - startBuild
				mu.Lock()
				server.buildProofsTime += buildTime
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

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
