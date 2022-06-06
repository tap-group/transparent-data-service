package tables

type ITableFactory interface {
	CreateTableWithoutRandomMissing(filename string)

	CreateTableWithRandomMissing(filename string)

	CreateTableForExperiment(filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, industryFeq, maxPower, mode, nonce int)

	RegenerateTableForExperiment(filename string, nTimeslots, startTime, missFreq, maxPower, mode, nonce int)

	CreateExample2Table(filename string)
}
