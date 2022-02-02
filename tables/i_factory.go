package tables

type ITableFactory interface {
	CreateTableWithoutRandomMissing(filename string)

	CreateTableWithRandomMissing(filename string)

	CreateTableWithRandomMissingForExperiment(filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, maxPower, mode int)

	RegenerateTableWithRandomMissingForExperiment(filename string, nTimeslots, startTime, missFreq, maxPower, mode int)

	RegenerateTableWithoutRandomMissingForExperiment(filename string, nTimeslots, startTime, maxPower, mode int)

	CreateExample2Table(filename string)
}
