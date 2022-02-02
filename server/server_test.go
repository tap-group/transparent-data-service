package server

import (
	"fmt"
	"math"
	"testing"

	"github.com/tap-group/tdsvc/tables"
)

func TestServerBuildTree(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(Server)
	columnNames := server.InitializeSqlTable(filename, tablename)

	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	fmt.Print("storage cost: ")
	fmt.Print(float64(1.*server.GetStorageCost()) / 1000000)
	fmt.Println(" MB")
}

func TestSizeCompare(t *testing.T) {
	nUsers := 10
	nDistricts := 10
	nEpochs := 100

	filename := "../table1.txt"
	tablename := "Table1"

	// add nEpochs entries in one go
	factory := new(tables.TableFactory)
	factory.CreateTableWithRandomMissingForExperiment(filename, nUsers, nDistricts, nEpochs, 0, 20, 2000, tables.UNIF_ZERO_TO_ND)

	server := new(Server)
	columnNames := server.InitializeSqlTable(filename, tablename)

	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	fmt.Print("storage cost 1: ")
	fmt.Print(float64(1.*server.GetStorageCost()) / 1000000)
	fmt.Println(" MB")
	storageCost1 := server.GetStorageCost()

	// add nEpochs entries step-by-step
	factory = new(tables.TableFactory)
	factory.CreateTableWithRandomMissingForExperiment(filename, nUsers, nDistricts, 1, 0, 20, 2000, tables.UNIF_ZERO_TO_ND)
	server = new(Server)
	columnNames = server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	for i := 1; i < nEpochs; i++ {
		valRange := [][]uint32{{uint32(i), uint32(i + 1)}, {0, math.MaxInt32}, {0, math.MaxInt32}}
		factory.RegenerateTableWithRandomMissingForExperiment(filename, 1, i, 20, 2000, tables.UNIF_ZERO_TO_ND)
		columnNames = server.AddToSqlTable(filename, tablename)
		server.AddToTree(columnNames, tablename, valRange)
	}

	fmt.Print("storage cost 2: ")
	fmt.Print(float64(1.*server.GetStorageCost()) / 1000000)
	fmt.Println(" MB")
	storageCost2 := server.GetStorageCost()

	if storageCost1 != storageCost2 {
		t.Error()
	}
}
