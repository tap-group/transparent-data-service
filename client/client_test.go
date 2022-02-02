package client

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/util"
)

func processLookupResult(c *Client, b, expB bool, desc string, err error, t *testing.T) {
	if err != nil {
		t.Error(err)
	}
	fmt.Print(desc + " lookup result: ")
	fmt.Print(b)
	fmt.Print(" (should be ")
	fmt.Print(expB)
	fmt.Print(") --- ")
	if b != expB {
		t.Error(err)
	}
	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*c.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
	c.ResetBandwidthUse()
}

func TestLookupQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	// test the first n entries
	maxN := 10
	for i := 1; i < maxN; i++ {
		entry := util.ReadCsvFileEntry(filename, i)
		if entry == nil {
			fmt.Println("warning: fewer than " + strconv.Itoa(i) + " entries in data file - terminating test")
			return
		}
		queryResult, err := client.LookupQuery(entry[0], entry[3], entry[4], entry[6], []uint32{entry[1], entry[2]})
		processLookupResult(client, queryResult, true, "entry "+strconv.Itoa(i), err, t)
	}
	// test fake entries - replace the measurement with something different (commitment tree inclusion proof should fail): assumes that measurements are below z
	z := 1000000
	for i := 1; i < maxN; i++ {
		entry := util.ReadCsvFileEntry(filename, i)
		if entry == nil {
			fmt.Println("warning: fewer than " + strconv.Itoa(i) + " entries in data file - terminating test")
			return
		}
		queryResult, err := client.LookupQuery(entry[0], entry[3], uint32(z+rand.Intn(z)), entry[6], []uint32{entry[1], entry[2]}) // replace the measurerment with a fake value
		processLookupResult(client, queryResult, false, "entry "+strconv.Itoa(i), err, t)
	}
	// test fake entries - replace the time slot with something different (prefix tree inclusion proof should fail): assumes that time slots are below z
	for i := 1; i < maxN; i++ {
		entry := util.ReadCsvFileEntry(filename, i)
		if entry == nil {
			fmt.Println("warning: fewer than " + strconv.Itoa(i) + " entries in data file - terminating test")
			return
		}
		fakeTime := z + rand.Intn(z)
		queryResult, err := client.LookupQuery(entry[0], uint32(fakeTime), entry[4], entry[6], []uint32{entry[1], entry[2]}) // replace the measurerment with a fake value
		processLookupResult(client, queryResult, false, "entry "+strconv.Itoa(i), err, t)
	}
}
func TestSumQueryExample1(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {2, 4}, {0, 2}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	sumResult, err := client.SumQuery(valRange)

	if err != nil {
		t.Error(err)
	}

	if sumResult <= 0 {
		t.Error()
	}

	fmt.Print("sum result: ")
	fmt.Println(sumResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestSumQueryExample2(t *testing.T) {
	filename := "../table_example2.txt"
	tablename := "Table2"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 1, 2, 3, nil)

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{13, 32}} // all values with time range 15 to 31

	sumResult, err := client.SumQuery(valRange)

	if err != nil {
		t.Error(err)
	}

	if sumResult <= 0 {
		t.Error()
	}

	fmt.Print("sum result: ")
	fmt.Println(sumResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestAvgQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {0, 5}, {0, 1}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	avgResult, err := client.AvgQuery(valRange)

	if err != nil {
		t.Error(err)
	}

	if avgResult <= 0 {
		t.Error()
	}

	fmt.Print("avg result: ")
	fmt.Println(avgResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestMinQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {0, 5}, {0, 1}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	minResult, err := client.MinQuery(valRange)

	if err != nil {
		t.Error(err)
	}

	if minResult <= 0 {
		t.Error()
	}

	fmt.Print("min result: ")
	fmt.Println(minResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestMaxQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {0, 5}, {0, 1}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	maxResult, err := client.MaxQuery(valRange)

	if err != nil {
		t.Error(err)
	}

	if maxResult <= 0 {
		t.Error()
	}

	fmt.Print("max result: ")
	fmt.Println(maxResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestMedianQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {0, 5}, {0, 1}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	medianResult, err := client.QuantileQuery(valRange, 1, 2)

	if err != nil {
		t.Error(err)
	}

	if medianResult <= 0 {
		t.Error()
	}

	fmt.Print("median result: ")
	fmt.Println(medianResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}

func TestQuartilesQuery(t *testing.T) {
	filename := "../table1.txt"
	tablename := "Table1"

	server := new(server.Server)
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(Client)
	client.SetServer(server)

	valRange := [][]uint32{{10, 20}, {0, 5}, {0, 1}} // all values with time range 10 to 20, district 0 to 4, is_industrial 0 to 1

	firstQuartileResult, err1 := client.QuantileQuery(valRange, 1, 4)
	medianResult, err2 := client.QuantileQuery(valRange, 2, 4)
	thirdQuartileResult, err3 := client.QuantileQuery(valRange, 3, 4)

	util.Check(err1)
	util.Check(err2)
	util.Check(err3)

	if firstQuartileResult <= 0 {
		t.Error()
	}
	fmt.Print("first quartile result: ")
	fmt.Println(firstQuartileResult)
	if medianResult <= 0 {
		t.Error()
	}
	fmt.Print("median result: ")
	fmt.Println(medianResult)
	if thirdQuartileResult <= 0 {
		t.Error()
	}
	fmt.Print("third quartile result: ")
	fmt.Println(thirdQuartileResult)

	fmt.Print("bandwidth cost: ")
	fmt.Print(float64(1.*client.GetBandwidthUse()) / 1000000)
	fmt.Println(" MB")
}
