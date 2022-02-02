package server

type IServer interface {
	GetLatestTimestamps() (int64, int64, int64, int64)

	GetPrefixTreeRootHash() []byte

	GetStorageCost() int

	AddToSqlTable(filename string, tableName string) []string

	InitializeSqlTable(filename string, tableName string) []string

	AddToTree(columnNames []string, tableName string, valRange [][]uint32)

	InitializeTree(
		columnNames []string, tableName string,
		userRowIdx, timeRowIdx, dataRowIdx, seedRowIdx uint32,
		miscFieldsIdxs []uint32,
	)

	RequestPrefixMembershipProof(prefix []byte) []byte

	RequestCommitmentTreeMembershipProof(inputBytes []byte) []byte

	RequestCommitmentsAndProofs(prefix []byte) []byte

	RequestPrefixes(valBytes []byte) []byte

	RequestSumAndCounts(queryPrefixes [][]byte) (int, int, [][][]byte, []int, [][]byte)

	RequestMinAndProofs(queryPrefixInputBytes []byte) (int, int, []byte, [][]byte, [][]byte)

	RequestMaxAndProofs(queryPrefixes [][]byte) (int, int, []byte, [][]byte, [][]byte)

	RequestQuantileAndProofs(queryPrefixes [][]byte, k int, q int) (int, int, [][]byte, [][]byte, [][]byte, [][]byte)

	ResetDurations()
}
