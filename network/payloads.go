package network

type TimestampsResponse struct {
	R0 int64
	R1 int64
	R2 int64
	R3 int64
}

type SqlTableRequest struct {
	Filename string
	Table    string
}

type AddToTreeRequest struct {
	Table      string
	Columns    []string
	ValueRange [][]uint32
}

type InitializeTreeRequest struct {
	Table          string
	Columns        []string
	UserRowIdx     uint32
	TimeRowIdx     uint32
	DataRowIdx     uint32
	SeedRowIdx     uint32
	MiscFieldsIdxs []uint32
}

// int, int, [][][]byte, []int, [][]byte,
type SumAndCountsResponse struct {
	R0 int
	R1 int
	R2 [][][]byte
	R3 []int
	R4 [][]byte
}

// int, int, []byte, [][]byte, [][]byte,
type MinMaxProofResponse struct {
	R0 int
	R1 int
	R2 []byte
	R3 [][]byte
	R4 [][]byte
}

// (queryPrefixes [][]byte, k int, q int)
type QuantileProofRequest struct {
	Prefixes [][]byte
	K        int
	Q        int
}

// int, int, [][]byte, [][]byte, [][]byte, [][]byte,
type QuantileProofResponse struct {
	R0 int
	R1 int
	R2 [][]byte
	R3 [][]byte
	R4 [][]byte
	R5 [][]byte
}

// filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, maxPower, mode int
type CreateTableRequest struct {
	Filename   string
	NUsers     int
	NDistricts int
	NTimeslots int
	StartTime  int
	MissFreq   int
	MaxPower   int
	Mode       int
}

// filename string, nTimeslots, startTime, missFreq, maxPower, mode int
type RegenerateTableRequest struct {
	Filename   string
	NTimeslots int
	StartTime  int
	MissFreq   int
	MaxPower   int
	Mode       int
}
