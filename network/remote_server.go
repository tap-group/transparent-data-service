package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tap-group/tdsvc/server"
)

type RemoteServer struct {
	host string
}

func NewRemoteServer(host string) *RemoteServer {
	rs := &RemoteServer{
		host: host,
	}
	rs.ping()
	return rs
}

var _ server.IServer = (*RemoteServer)(nil)

func (rs *RemoteServer) ping() {
	rs.sendRequest(PingEndpoint, nil)
}

func (rs *RemoteServer) GetLatestTimestamps() (int64, int64, int64, int64) {
	data := rs.sendRequest(GetLatestTimestampsEndpoint, nil)
	var ret TimestampsResponse
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret.R0, ret.R1, ret.R2, ret.R3
}

func (rs *RemoteServer) GetPrefixTreeRootHash() []byte {
	data := rs.sendRequest(GetPrefixTreeRootHashEndpoint, nil)
	var ret []byte
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) GetStorageCost() int {
	data := rs.sendRequest(GetStorageCostEndpoint, nil)
	var ret int
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) AddToSqlTable(filename string, tableName string) []string {
	payload, _ := json.Marshal(SqlTableRequest{
		Filename: filename,
		Table:    tableName,
	})
	data := rs.sendRequest(AddToSqlTableEndpoint, payload)
	var ret []string
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) InitializeSqlTable(filename string, tableName string) []string {
	payload, _ := json.Marshal(SqlTableRequest{
		Filename: filename,
		Table:    tableName,
	})
	data := rs.sendRequest(InitializeSqlTableEndpoint, payload)
	var ret []string
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) AddToTree(columnNames []string, tableName string, valRange [][]uint32) {
	payload, _ := json.Marshal(AddToTreeRequest{
		Table:      tableName,
		Columns:    columnNames,
		ValueRange: valRange,
	})
	rs.sendRequest(AddToTreeEndpoint, payload)
}

func (rs *RemoteServer) InitializeTree(
	columnNames []string, tableName string,
	userRowIdx, timeRowIdx, dataRowIdx, seedRowIdx uint32,
	miscFieldsIdxs []uint32,
) {
	payload, _ := json.Marshal(InitializeTreeRequest{
		Table:          tableName,
		Columns:        columnNames,
		UserRowIdx:     userRowIdx,
		TimeRowIdx:     timeRowIdx,
		DataRowIdx:     dataRowIdx,
		SeedRowIdx:     seedRowIdx,
		MiscFieldsIdxs: miscFieldsIdxs,
	})
	rs.sendRequest(InitializeTreeEndpoint, payload)
}

func (rs *RemoteServer) RequestPrefixMembershipProof(prefix []byte) []byte {
	payload, _ := json.Marshal(prefix)
	data := rs.sendRequest(RequestPrefixMembershipProofEndpoint, payload)
	var ret []byte
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) RequestCommitmentTreeMembershipProof(inputBytes []byte) []byte {
	payload, _ := json.Marshal(inputBytes)
	data := rs.sendRequest(RequestCommitmentTreeMembershipProofEndpoint, payload)
	var ret []byte
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) RequestCommitmentsAndProofs(prefix []byte) []byte {
	payload, _ := json.Marshal(prefix)
	data := rs.sendRequest(RequestCommitmentsAndProofsEndpoint, payload)
	var ret []byte
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) RequestPrefixes(valBytes []byte) []byte {
	payload, _ := json.Marshal(valBytes)
	data := rs.sendRequest(RequestPrefixesEndpoint, payload)
	var ret []byte
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret
}

func (rs *RemoteServer) RequestSumAndCounts(queryPrefixes [][]byte) (
	int, int, [][][]byte, []int, [][]byte,
) {
	payload, _ := json.Marshal(queryPrefixes)
	data := rs.sendRequest(RequestSumAndCountsEndpoint, payload)
	var ret SumAndCountsResponse
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret.R0, ret.R1, ret.R2, ret.R3, ret.R4
}

func (rs *RemoteServer) RequestMinAndProofs(queryPrefixInputBytes []byte) (
	int, int, []byte, [][]byte, [][]byte,
) {
	payload, _ := json.Marshal(queryPrefixInputBytes)
	data := rs.sendRequest(RequestMinAndProofsEndpoint, payload)
	var ret MinMaxProofResponse
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret.R0, ret.R1, ret.R2, ret.R3, ret.R4
}

func (rs *RemoteServer) RequestMaxAndProofs(queryPrefixes [][]byte) (
	int, int, []byte, [][]byte, [][]byte,
) {
	payload, _ := json.Marshal(queryPrefixes)
	data := rs.sendRequest(RequestMaxAndProofsEndpoint, payload)
	var ret MinMaxProofResponse
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret.R0, ret.R1, ret.R2, ret.R3, ret.R4
}

func (rs *RemoteServer) RequestQuantileAndProofs(queryPrefixes [][]byte, k int, q int) (
	int, int, [][]byte, [][]byte, [][]byte, [][]byte,
) {
	payload, _ := json.Marshal(QuantileProofRequest{
		Prefixes: queryPrefixes,
		K:        k,
		Q:        q,
	})
	data := rs.sendRequest(RequestQuantileAndProofsEndpoint, payload)
	var ret QuantileProofResponse
	err := json.Unmarshal(data, &ret)
	check(err)
	return ret.R0, ret.R1, ret.R2, ret.R3, ret.R4, ret.R5
}

func (rs *RemoteServer) ResetDurations() {
	rs.sendRequest(ResetDurationsEndpoint, nil)
}

func (rs *RemoteServer) sendRequest(endpoint string, payload []byte) []byte {
	resp, err := http.Post(rs.host+endpoint, "application/json", bytes.NewReader(payload))
	check(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		check(fmt.Errorf("request failed. endpoint=%s status = %d", endpoint, resp.StatusCode))
	}
	ret, err := ioutil.ReadAll(resp.Body)
	check(err)
	return ret
}

// panic if error occurs
func check(err error) {
	if err != nil {
		panic(fmt.Errorf("network error: %+v", err))
	}
}
