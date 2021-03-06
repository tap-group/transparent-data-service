package network

import (
	"encoding/json"

	"github.com/tap-group/tdsvc/tables"
)

type RemoteTableFactory struct {
	server *RemoteServer
}

var _ tables.ITableFactory = (*RemoteTableFactory)(nil)

func NewRemoteFactory(server *RemoteServer) *RemoteTableFactory {
	return &RemoteTableFactory{
		server: server,
	}
}

func (rf *RemoteTableFactory) CreateTableWithoutRandomMissing(filename string) {
	payload, _ := json.Marshal(filename)
	rf.server.sendRequest(CreateTableWithoutRandomMissingEndpoint, payload)
}

func (rf *RemoteTableFactory) CreateTableWithRandomMissing(filename string) {
	payload, _ := json.Marshal(filename)
	rf.server.sendRequest(CreateTableWithRandomMissingEndpoint, payload)
}

func (rf *RemoteTableFactory) CreateTableForExperiment(
	filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, IndustryFreq, maxPower, mode, nonce int,
) {
	payload, _ := json.Marshal(CreateTableRequest{
		Filename:     filename,
		NUsers:       nUsers,
		NDistricts:   nDistricts,
		NTimeslots:   nTimeslots,
		StartTime:    startTime,
		MissFreq:     missFreq,
		IndustryFreq: IndustryFreq,
		MaxPower:     maxPower,
		Mode:         mode,
		Nonce:        nonce,
	})
	rf.server.sendRequest(CreateTableForExperimentEndpoint, payload)
}

func (rf *RemoteTableFactory) RegenerateTableForExperiment(filename string, nTimeslots, startTime, missFreq, maxPower, mode, nonce int) {
	payload, _ := json.Marshal(RegenerateTableRequest{
		Filename:   filename,
		NTimeslots: nTimeslots,
		StartTime:  startTime,
		MaxPower:   maxPower,
		Mode:       mode,
		Nonce:      nonce,
	})
	rf.server.sendRequest(RegenerateTableForExperimentEndpoint, payload)
}

func (rf *RemoteTableFactory) CreateExample2Table(filename string) {
	payload, _ := json.Marshal(filename)
	rf.server.sendRequest(CreateExample2TableEndpoint, payload)
}
