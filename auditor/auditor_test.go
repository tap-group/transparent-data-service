package auditor

import (
	"fmt"
	"math"
	"testing"

	"github.com/tap-group/tdsvc/server"
)

func TestPrefixTreeCheck(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	auditor := new(Auditor)
	auditor.SetServer(server)
	prefixTreeCheck := auditor.CheckPrefixTree(3) // time + two misc. val columns

	if !prefixTreeCheck {
		t.Error()
	}

	fmt.Print("auditor storage cost: ")
	fmt.Print(float64(1.*auditor.GetStorageCost()) / 1000000)
	fmt.Println(" MB")
	fmt.Print("auditor bandwidth cost: ")
	fmt.Print(float64(1.*auditor.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestCommitmentTreesCheck(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	// valid to check a subset of the commitment trees
	valRange := [][]uint32{{0, 10}, {0, math.MaxInt32}, {0, math.MaxInt32}}

	auditor := new(Auditor)
	auditor.SetServer(server)
	commitmentTreeCheck := auditor.CheckAllCommitTrees(valRange)

	if !commitmentTreeCheck {
		t.Error()
	}

	fmt.Print("auditor storage cost: ")
	fmt.Print(float64(1.*auditor.GetStorageCost()) / 1000000)
	fmt.Println(" MB")
	fmt.Print("auditor bandwidth cost: ")
	fmt.Print(float64(1.*auditor.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}
