package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/tap-group/tdsvc/auditor"
	"github.com/tap-group/tdsvc/client"
	"github.com/tap-group/tdsvc/network"
	pkg_server "github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/tables"
	"github.com/tap-group/tdsvc/util"
)

const INSERT = 0
const LOOKUP = 1
const SUM = 2
const AVG = 3
const COUNT = 4
const MIN = 5
const MAX = 6
const MEDIAN = 7
const PERCENTILE_5 = 8

var (
	server  pkg_server.IServer
	factory tables.ITableFactory
)

func processTimeDiff(timeDiff int64) string {
	return fmt.Sprintf("%f", float64(timeDiff)/1000000000)
}

func writeCsvFile(filename string, colNames []string, data [][]string) {
	file, err := os.Create(filename)
	util.Check(err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write(colNames)
	util.Check(err)
	for _, value := range data {
		err := writer.Write(value)
		util.Check(err)
	}
}

func runServerDataInsertionCostsPerEpoch(nEpochs int, nUsers int, nDistricts int, outputfilename string, mode int) {
	filename := "input/table1.txt"
	tablename := "Table1"

	factory.CreateTableForExperiment(filename, nUsers, nDistricts, 1, 0, 20, 5, 2000, mode, 0) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power

	start := time.Now().UnixNano()
	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})
	end := time.Now().UnixNano()

	insertCosts := [][]string{{"0", processTimeDiff(end - start), strconv.Itoa(server.GetStorageCost())}}
	fmt.Print("updating tree: ")
	sumTime := int64(0)
	sumN := 0
	for n := 0; n < nEpochs; n++ {
		factory.RegenerateTableForExperiment(filename, 1, n+1, 0, 2000, mode, 0)
		// start = time.Now().UnixNano()
		columnNames = server.AddToSqlTable(filename, tablename)
		// end = time.Now().UnixNano()
		// fmt.Print(processTimeDiff(end-start) + ", ")

		valRange := [][]uint32{{uint32(n + 1), uint32(n + 2)}, {0, math.MaxInt32}, {0, math.MaxInt32}} // only check time slot n+1, and all other entries

		start = time.Now().UnixNano()
		server.AddToTree(columnNames, tablename, valRange)
		end = time.Now().UnixNano()

		sumTime += end - start
		sumN++

		if n%(nEpochs/100) == 0 {
			avgTime := fmt.Sprintf("%f", float64(sumTime)/float64(sumN)/1000000000)
			// result := []string{strconv.Itoa(n + 1), avgTime, "0"}
			result := []string{strconv.Itoa(n + 1), avgTime, strconv.Itoa(server.GetStorageCost())} // the time cost of GetStorageCost can be high, especially for larger trees
			insertCosts = append(insertCosts, result)
			fmt.Println(result)

			fmt.Print(strconv.Itoa(n) + "/" + strconv.Itoa(nEpochs) + ", ")
		}
	}
	fmt.Println()

	fmt.Print("results: ")
	fmt.Println(insertCosts)
	writeCsvFile(outputfilename, []string{"epoch", "timecost", "storagecost"}, insertCosts)
}

func runClientQueryCostsPerEpoch(nEpochs int, nUsers int, nDistricts int, outputfilename string, mode int, query int) {
	filename := "input/table3.txt"
	tablename := "Table3"

	factory.CreateTableForExperiment(filename, nUsers, nDistricts, 1, 0, 20, 5, 200, mode, 0) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power

	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(client.Client)
	client.SetServer(server)

	sumTime := int64(0)
	sumBW := 0
	sumN := 0

	result := [][]string{}

	for n := 0; n < nEpochs; n++ {
		factory.RegenerateTableForExperiment(filename, 1, n+1, 200, 0, mode, 0)
		columnNames = server.AddToSqlTable(filename, tablename)
		valRange := [][]uint32{{uint32(n + 1), uint32(n + 2)}, {0, math.MaxInt32}, {0, math.MaxInt32}} // only check time slot n+1, and all other entries
		server.AddToTree(columnNames, tablename, valRange)

		startLookup := time.Now().UnixNano()
		// execute the query
		var err error
		queryValRange := [][]uint32{{uint32(n), uint32(n + 10)}, {0, math.MaxInt32}, {0, 1}}
		if query == LOOKUP {
			entry := util.ReadCsvFileEntry(filename, rand.Intn(10)+1)
			_, err = client.LookupQuery(entry[0], entry[3], entry[4], entry[6], []uint32{entry[1], entry[2]})
		} else if query == SUM {
			_, err = client.SumQuery(queryValRange)
		} else if query == AVG {
			_, err = client.AvgQuery(queryValRange)
		} else if query == COUNT {
			_, err = client.CountQuery(queryValRange)
		} else if query == MIN {
			_, err = client.MinQuery(queryValRange)
		} else if query == MAX {
			_, err = client.MaxQuery(queryValRange)
		} else if query == MEDIAN {
			_, err = client.QuantileQuery(queryValRange, 1, 2)
		} else if query == PERCENTILE_5 {
			_, err = client.QuantileQuery(queryValRange, 5, 100)
		}
		// check for errors
		if err != nil {
			fmt.Println("query failed -- terminating experiment")
			log.Fatal(err)
		}
		endLookup := time.Now().UnixNano()

		sumTime += endLookup - startLookup
		sumBW += client.GetBandwidthUse()
		sumN++

		if n%(nEpochs/100) == 0 {
			avgTime := fmt.Sprintf("%f", float64(sumTime)/float64(sumN)/1000000000)
			avgBW := fmt.Sprintf("%f", float64(sumBW)/float64(sumN)/1000000)
			result = append(result, []string{strconv.Itoa(n), avgTime, avgBW})
			fmt.Print(strconv.Itoa(n) + "/" + strconv.Itoa(nEpochs) + ", ")
			fmt.Print(avgTime)
			fmt.Print(", ")
			fmt.Println(avgBW)
		}
		client.ResetBandwidthUse()
	}
	fmt.Println()

	fmt.Print("results: ")
	fmt.Println(result)
	writeCsvFile(outputfilename, []string{"epoch", "timecost", "bandwidthcost"}, result)
}

func runClientQueryCostsSingleEpoch(nEpochs int, nUsers int, nDistricts int, nRepeats int, mode int, query int, idx int) []string {
	filename := "input/table4.txt"
	tablename := "Table4"

	factory.CreateTableForExperiment(filename, nUsers, nDistricts, nEpochs, 0, 20, 5, 200, mode, 0) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power

	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(client.Client)
	client.SetServer(server)

	sums := make([]int64, 9)

	for i := 0; i < nRepeats; i++ {
		startTime := time.Now().UnixNano()
		// execute the query
		var err error
		queryValRange := [][]uint32{{uint32(0), uint32(10)}, {0, math.MaxInt32}, {0, 1}}
		if query == LOOKUP {
			entry := util.ReadCsvFileEntry(filename, rand.Intn(10)+1)
			_, err = client.LookupQuery(entry[0], entry[3], entry[4], entry[6], []uint32{entry[1], entry[2]})
		} else if query == SUM {
			_, err = client.SumQuery(queryValRange)
		} else if query == AVG {
			_, err = client.AvgQuery(queryValRange)
		} else if query == COUNT {
			_, err = client.CountQuery(queryValRange)
		} else if query == MIN {
			_, err = client.MinQuery(queryValRange)
		} else if query == MAX {
			_, err = client.MaxQuery(queryValRange)
		} else if query == MEDIAN {
			_, err = client.QuantileQuery(queryValRange, 1, 2)
		} else if query == PERCENTILE_5 {
			_, err = client.QuantileQuery(queryValRange, 5, 100)
		}
		// check for errors
		if err != nil {
			fmt.Println("query failed -- terminating experiment")
			log.Fatal(err)
		}
		endTime := time.Now().UnixNano()

		prefixReceiveRequestTime, prefixSendResponseTime, queryReceiveRequestTime, querySendResponseTime := server.GetLatestTimestamps()
		prefixSendRequestTime, prefixReceiveResponseTime, querySendRequestTime, queryReceiveResponseTime, finishTime := client.GetLatestTimestamps()

		sums[0] += prefixReceiveRequestTime - prefixSendRequestTime
		sums[1] += prefixSendResponseTime - prefixReceiveRequestTime
		sums[2] += prefixReceiveResponseTime - prefixSendResponseTime
		sums[3] += querySendRequestTime - prefixReceiveResponseTime
		sums[4] += queryReceiveRequestTime - querySendRequestTime
		sums[5] += querySendResponseTime - queryReceiveRequestTime
		sums[6] += queryReceiveResponseTime - querySendResponseTime
		sums[7] += finishTime - queryReceiveResponseTime
		sums[8] += endTime - startTime
	}

	nRepeats64 := int64(nRepeats)

	return []string{
		strconv.Itoa(idx),
		strconv.FormatInt(sums[0]/nRepeats64, 10),
		strconv.FormatInt(sums[1]/nRepeats64, 10),
		strconv.FormatInt(sums[2]/nRepeats64, 10),
		strconv.FormatInt(sums[3]/nRepeats64, 10),
		strconv.FormatInt(sums[4]/nRepeats64, 10),
		strconv.FormatInt(sums[5]/nRepeats64, 10),
		strconv.FormatInt(sums[6]/nRepeats64, 10),
		strconv.FormatInt(sums[7]/nRepeats64, 10),
		strconv.FormatInt(sums[8]/nRepeats64, 10),
	}
}

func runTableEntry(nEpochs int, nUsers int, nDistricts int, nRepeats int, mode int) []string {
	fmt.Println("---------")
	filename := "input/table4.txt"
	tablename := "Table4"

	factory := new(tables.TableFactory)
	factory.CreateTableForExperiment(filename, nUsers, nDistricts, nEpochs, 0, 20, 5, 2000, mode, 0) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power

	columnNames := server.InitializeSqlTable(filename, tablename)
	server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})

	client := new(client.Client)
	client.SetServer(server)

	auditor := new(auditor.Auditor)
	auditor.SetServer(server)

	sums := make([]int64, 7)
	var (
		startTime int64
		endTime   int64
	)

	// storage cost
	sums[0] = int64(server.GetStorageCost())

	for i := 0; i < nRepeats; i++ {
		// insert
		insertValRange := [][]uint32{{uint32(nEpochs + i + 1), uint32(nEpochs + i + 2)}, {0, math.MaxInt32}, {0, math.MaxInt32}} // only check time slot i+1, and all other entries
		factory.RegenerateTableForExperiment(filename, 1, nEpochs+i+1, 0, 2000, mode, 0)
		columnNames = server.AddToSqlTable(filename, tablename)

		server.ResetDurations()

		startTime = time.Now().UnixNano()
		server.AddToTree(columnNames, tablename, insertValRange)
		endTime = time.Now().UnixNano()
		sums[1] += endTime - startTime

		// client look-up (for monitoring)
		entry := util.ReadCsvFileEntry(filename, rand.Intn(10)+1)

		server.ResetDurations()

		startTime = time.Now().UnixNano()
		client.LookupQuery(entry[0], entry[3], entry[4], entry[6], []uint32{entry[1], entry[2]})
		endTime = time.Now().UnixNano()
		sums[2] += endTime - startTime

		// auditor monitoring
		startTime := time.Now().UnixNano()
		auditor.CheckPrefixTree(3)
		auditor.CheckAllCommitTrees(insertValRange)
		endTime := time.Now().UnixNano()
		sums[3] += endTime - startTime

		// queries
		queryValRange := [][]uint32{{uint32(0), uint32(10)}, {0, math.MaxInt32}, {0, 1}}

		startTime = time.Now().UnixNano()
		client.SumQuery(queryValRange)
		endTime = time.Now().UnixNano()
		sums[4] += endTime - startTime

		startTime = time.Now().UnixNano()
		client.MinQuery(queryValRange)
		endTime = time.Now().UnixNano()
		sums[5] += endTime - startTime

		startTime = time.Now().UnixNano()
		client.QuantileQuery(queryValRange, 1, 2)
		endTime = time.Now().UnixNano()
		sums[6] += endTime - startTime
	}

	return []string{
		strconv.FormatInt(sums[0], 10),
		fmt.Sprintf("%.3f", float64(sums[1])/(float64(nRepeats)*float64(1000000000))),
		fmt.Sprintf("%.3f", float64(sums[2])/(float64(nRepeats)*float64(1000000000))),
		fmt.Sprintf("%.3f", float64(sums[3])/(float64(nRepeats)*float64(1000000000))),
		fmt.Sprintf("%.3f", float64(sums[4])/(float64(nRepeats)*float64(1000000000))),
		fmt.Sprintf("%.3f", float64(sums[5])/(float64(nRepeats)*float64(1000000000))),
		fmt.Sprintf("%.3f", float64(sums[6])/(float64(nRepeats)*float64(1000000000))),
	}
}

func getQueryRunTimes(client *client.Client, queryValRange [][]uint32, query int) (int64, int64) {
	startTime := time.Now().UnixNano()
	var err error
	if query == 0 {
		_, err = client.SumQuery(queryValRange)
	} else if query == 1 {
		_, err = client.MinQuery(queryValRange)
	} else if query == 2 {
		_, err = client.QuantileQuery(queryValRange, 1, 2)
	}
	util.Check(err)
	totalTime := time.Now().UnixNano() - startTime
	clientTime := client.GetProcessingTime()
	return totalTime, clientTime
}

func getNSamplesForNumUsers(n, u int) int {
	if n > 0 {
		return n
	}
	if u < 1000 {
		return 25
	}
	if u > 1000000 {
		return 1
	}
	return 5
}

func runQueryScalabilityExperiment(nUsers []int, nDistricts []int, nSamples, mode int) {
	inputFilename := "input/table6.txt"
	outputFilename := "output/experiment6.csv"
	tablename := "Table6"

	file, err := os.OpenFile(outputFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	util.Check(err)
	datawriter := bufio.NewWriter(file)
	_, _ = datawriter.WriteString("n_users, n_trees, file, sql, tree, lookup_total, lookup_client, sum_all_total, sum_all_client, min_all_total, min_all_client, quantile_all_total, quantile_all_client, sum_limited_total, sum_limited_client, min_limited_total, min_limited_client, quantile_limited_total, quantile_limited_client\n")

	printExperimentRange(nUsers, nDistricts)

	for _, u := range nUsers {
		for _, d := range nDistricts {
			timeResults := make([]int64, 17)
			for i := range timeResults {
				timeResults[i] = 0
			}

			n := getNSamplesForNumUsers(nSamples, u)

			for j := 0; j < n; j++ {
				fmt.Println(strconv.Itoa(u) + " users, " + strconv.Itoa(d) + " subtrees, sample " + strconv.Itoa(j+1) + ":")

				// create the data file
				startFile := time.Now().UnixNano()
				factory.CreateTableForExperiment(inputFilename, u, d, 1, 0, 0, 0, 200, mode, j) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power
				endFile := time.Now().UnixNano()
				timeResults[0] += endFile - startFile
				fmt.Print("generating data file: ")
				fmt.Print(float64(endFile-startFile) / 1000000000)
				fmt.Println("s")
				// sql table
				columnNames := server.InitializeSqlTable(inputFilename, tablename)
				endSql := time.Now().UnixNano()
				timeResults[1] += endSql - endFile
				fmt.Print("adding data to sql table: ")
				fmt.Print(float64(endSql-endFile) / 1000000000)
				fmt.Println("s")
				// build the tree to determine the root
				server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})
				endTree := time.Now().UnixNano()
				timeResults[2] += endTree - endSql
				fmt.Print("creating TAP data structure: ")
				fmt.Print(float64(endTree-endSql) / 1000000000)
				fmt.Println("s")

				// client-server queries
				client := new(client.Client)
				client.SetServer(server)

				var err error
				// lookup query
				entry := util.ReadCsvFileEntry(inputFilename, 1)
				startLookup := time.Now().UnixNano()
				_, err = client.LookupQuery(entry[0], entry[3], entry[4], entry[6], []uint32{entry[1], entry[2]})
				util.Check(err)
				endLookup := time.Now().UnixNano()
				lookupTime := endLookup - startLookup
				clientLookupTime := client.GetProcessingTime()
				timeResults[3] += lookupTime
				timeResults[4] += clientLookupTime

				// query over full range
				queryValRange := [][]uint32{{0, 1}, {0, math.MaxInt32}, {0, math.MaxInt32}}
				totalSumTime, clientSumTime := getQueryRunTimes(client, queryValRange, 0)
				timeResults[5] += totalSumTime
				timeResults[6] += clientSumTime
				totalMinTime, clientMinTime := getQueryRunTimes(client, queryValRange, 1)
				timeResults[7] += totalMinTime
				timeResults[8] += clientMinTime
				totalQuantileTime, clientQuantileTime := getQueryRunTimes(client, queryValRange, 2)
				timeResults[9] += totalQuantileTime
				timeResults[10] += clientQuantileTime

				// query over limited range (first 10 subtrees)
				queryValRange = [][]uint32{{0, 1}, {0, 10}, {0, math.MaxInt32}}
				totalSumLimitedTime, clientSumLimitedTime := getQueryRunTimes(client, queryValRange, 0)
				timeResults[11] += totalSumLimitedTime
				timeResults[12] += clientSumLimitedTime
				totalMinLimitedTime, clientMinLimitedTime := getQueryRunTimes(client, queryValRange, 1)
				timeResults[13] += totalMinLimitedTime
				timeResults[14] += clientMinLimitedTime
				totalQuantileLimitedTime, clientQuantileLimitedTime := getQueryRunTimes(client, queryValRange, 2)
				timeResults[15] += totalQuantileLimitedTime
				timeResults[16] += clientQuantileLimitedTime
			}

			resultString := strconv.Itoa(u) + "," + strconv.Itoa(d)
			for i := range timeResults {
				resultString += "," + fmt.Sprint(float64(timeResults[i]/int64(n))/1000000000)
			}
			_, _ = datawriter.WriteString(resultString + "\n")
			datawriter.Flush()
		}
	}

	file.Close()
	fmt.Println()
}

func runAuditScalabilityExperiment(nUsers []int, nDistricts []int, nSamples, mode int) {
	inputFilename := "input/table7.txt"
	outputFilename := "output/experiment7.csv"
	tablename := "Table7"

	file, err := os.OpenFile(outputFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	util.Check(err)
	datawriter := bufio.NewWriter(file)
	_, _ = datawriter.WriteString("n_users, n_trees, file, sql, tree, audit_prefix_tree, audit_sum_trees\n")

	printExperimentRange(nUsers, nDistricts)

	for _, u := range nUsers {
		for _, d := range nDistricts {
			timeResults := make([]int64, 5)
			for i := range timeResults {
				timeResults[i] = 0
			}

			n := getNSamplesForNumUsers(nSamples, u)

			for j := 0; j < n; j++ {
				fmt.Println(strconv.Itoa(u) + " users, " + strconv.Itoa(d) + " subtrees, sample " + strconv.Itoa(j+1) + ":")

				// create the data file
				startFile := time.Now().UnixNano()
				factory.CreateTableForExperiment(inputFilename, u, d, 1, 0, 0, 0, 200, mode, j) // filename, nUsers, nDistricts, nTimeslots, start time, miss freq., max. power
				endFile := time.Now().UnixNano()
				timeResults[0] += endFile - startFile
				fmt.Print("generating data file: ")
				fmt.Print(float64(endFile-startFile) / 1000000000)
				fmt.Println("s")
				// sql table
				columnNames := server.InitializeSqlTable(inputFilename, tablename)
				endSql := time.Now().UnixNano()
				timeResults[1] += endSql - endFile
				fmt.Print("adding data to sql table: ")
				fmt.Print(float64(endSql-endFile) / 1000000000)
				fmt.Println("s")
				// build the tree to determine the root
				server.InitializeTree(columnNames, tablename, 0, 3, 4, 6, []uint32{1, 2})
				endTree := time.Now().UnixNano()
				timeResults[2] += endTree - endSql
				fmt.Print("creating TAP data structure: ")
				fmt.Print(float64(endTree-endSql) / 1000000000)
				fmt.Println("s")

				// client-server queries
				auditor := new(auditor.Auditor)
				auditor.SetServer(server)

				// prefix tree structure audit
				startPrefixTreeCheck := time.Now().UnixNano()
				auditor.CheckPrefixTree(3) // time + two misc. val columns
				endPrefixTreeCheck := time.Now().UnixNano()
				timeResults[3] += endPrefixTreeCheck - startPrefixTreeCheck

				// sum tree ordering audit
				valRange := [][]uint32{{0, 1}, {0, math.MaxInt32}, {0, math.MaxInt32}}
				startCommitmentTreesCheck := time.Now().UnixNano()
				auditor.CheckAllCommitTrees(valRange)
				endCommitmentTreesCheck := time.Now().UnixNano()
				timeResults[4] += endCommitmentTreesCheck - startCommitmentTreesCheck
			}

			resultString := strconv.Itoa(u) + "," + strconv.Itoa(d)
			for i := range timeResults {
				resultString += "," + fmt.Sprint(float64(timeResults[i]/int64(n))/1000000000)
			}
			_, _ = datawriter.WriteString(resultString + "\n")
			datawriter.Flush()
		}
	}

	file.Close()
	fmt.Println()
}

func runExperiment1a(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 1a: costs of inserting data at server for different #prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment1a.csv", tables.UNIF_ZERO_TO_ND)
}

func runExperiment1b(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 1b: costs of inserting data at server for different #prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment1b.csv", tables.UNIF_ZERO_TO_ND)
}

func runExperiment1c(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 1c: costs of inserting data at server for different #prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment1c.csv", tables.UNIF_ZERO_TO_ND)
}

func runExperiment2a(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 2a: costs of inserting data at server for different distribution of prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment2a.csv", tables.UNIF_ZERO_TO_ND)
}

func runExperiment2b(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 2b: costs of inserting data at server for different distribution of prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment2b.csv", tables.UNIF_MAX_TO_MAX_MINUS_ND)
}

func runExperiment2c(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 2c: costs of inserting data at server for different distribution of prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment2c.csv", tables.UNIF_ZERO_TO_MAX)
}

func runExperiment2d(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 2d: costs of inserting data at server for different distribution of prefix tree leaves\n")
	runServerDataInsertionCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment2d.csv", tables.RAND_ZERO_TO_MAX)
}

func runExperiment3a(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3a: costs of look-up at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3a.csv", tables.UNIF_ZERO_TO_ND, LOOKUP)
}

func runExperiment3b(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3b: costs of sum at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3b.csv", tables.UNIF_ZERO_TO_ND, SUM)
}

func runExperiment3c(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3c: costs of avg at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3c.csv", tables.UNIF_ZERO_TO_ND, AVG)
}

func runExperiment3d(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3d: costs of count at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3d.csv", tables.UNIF_ZERO_TO_ND, COUNT)
}

func runExperiment3e(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3e: costs of min at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3e.csv", tables.UNIF_ZERO_TO_ND, MIN)
}

func runExperiment3f(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3f: costs of max at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3f.csv", tables.UNIF_ZERO_TO_ND, MAX)
}

func runExperiment3g(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3g: costs of median at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3g.csv", tables.UNIF_ZERO_TO_ND, MEDIAN)
}

func runExperiment3h(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 3h: costs of 5th percentile at server and client\n")
	runClientQueryCostsPerEpoch(nEpochs, nUsers, nDistricts, "output/experiment3h.csv", tables.UNIF_ZERO_TO_ND, PERCENTILE_5)
}

func runExperiment4(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 4: time cost at client/server for different queries\n")

	results := make([][]string, 0)
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, LOOKUP, 0))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*3, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, LOOKUP, 1))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*10, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, LOOKUP, 2))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, SUM, 3))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*3, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, SUM, 4))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*10, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, SUM, 5))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MIN, 6))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*3, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MIN, 7))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*10, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MIN, 8))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MEDIAN, 9))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*3, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MEDIAN, 10))
	results = append(results, runClientQueryCostsSingleEpoch(nEpochs*10, nUsers, nDistricts, 5, tables.UNIF_ZERO_TO_ND, MEDIAN, 11))

	writeCsvFile("output/experiment4.csv", []string{"idx", "prefix_request_transfer_time", "prefix_proc_time_server", "prefix_response_transfer_time", "prefix_proc_time_client", "query_request_transfer_time", "query_proc_time_server", "query_response_transfer_time", "query_proc_time_client", "total_time"}, results)
}

func runExperiment5(nEpochs int, nUsers int, nDistricts int) {
	fmt.Printf("running experiment 5: table entries\n")
	results := make([][]string, 0)
	nRepeats := 2
	results = append(results, runTableEntry(1, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(2, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(5, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(10, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(20, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(40, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(80, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	results = append(results, runTableEntry(160, nUsers, nDistricts, nRepeats, tables.UNIF_ZERO_TO_ND))
	writeCsvFile("output/experiment5.csv", []string{"storage", "insert", "lookup", "auditor", "sum", "min", "quantile"}, results)
}

func printExperimentRange(nUsers, nDistricts []int) {
	fmt.Print("n. users: ")
	for _, users := range nUsers {
		fmt.Print(users)
		fmt.Print(", ")
	}
	fmt.Println()
	fmt.Print("n. districts: ")
	for _, districts := range nDistricts {
		fmt.Print(districts)
		fmt.Print(", ")
	}
	fmt.Println()
}

func getExperimentRange(usersMin, usersMax, usersNum, districtsMin, districtsMax, districtsNum int) ([]int, []int) {
	nUsers := make([]int, usersNum)
	nDistricts := make([]int, districtsNum)
	usersExp := math.Pow(float64(usersMax/usersMin), 1./float64(usersNum-1))
	districtsExp := math.Pow(float64(districtsMax/districtsMin), 1./float64(districtsNum-1))
	for i := 0; i < usersNum; i++ {
		nUsers[i] = int(math.Floor(float64(usersMin) * math.Pow(usersExp, float64(i))))
	}
	for i := 0; i < districtsNum; i++ {
		nDistricts[i] = int(math.Floor(float64(districtsMin) * math.Pow(districtsExp, float64(i))))
	}
	return nUsers, nDistricts
}

func runExperiment6(nEpochs int, nUsers, nDistricts []int, nSamples int) {
	fmt.Printf("running experiment 6: scalability test of queries\n")
	runQueryScalabilityExperiment(nUsers, nDistricts, nSamples, tables.RAND_ZERO_TO_ND)
}

func runExperiment7(nEpochs int, nUsers, nDistricts []int, nSamples int) {
	fmt.Printf("running experiment 7: scalability test of audits\n")
	runAuditScalabilityExperiment(nUsers, nDistricts, nSamples, tables.RAND_ZERO_TO_ND)
}

func main() {
	nUsers := 100
	// n. users: 100, 267, 715, 1912, 5113, 13673, 36565, 97780, 261476, 699216, 1869781, 5000000,
	// n. districts: 10, 21, 46, 100, 215, 464, 1000,
	nUsers6, nDistricts6 := getExperimentRange(100, 5000000, 12, 10, 1000, 3)
	// n. users: 100, 187, 351, 657, 1232, 2310, 4328, 8111, 15199, 28480, 53366, 100000,
	// n. districts: 10, 100,
	nUsers7, nDistricts7 := getExperimentRange(100, 100000, 12, 10, 100, 2) //
	nEpochs1 := 100
	nEpochs2 := 500
	nEpochs3 := 100
	nEpochs4 := 10
	nEpochs5 := 1
	nEpochs6 := 1
	nEpochs7 := 1
	nSamples6 := -1
	nSamples7 := 3

	remote := flag.Bool("remote", false, "run remote server")
	serverURL := flag.String("url", "http://localhost:9045", "remote server url")
	createTablesFlag := flag.Bool("create", false, "create new table")
	performExperiment1Flag := flag.Bool("experiment1", false, "perform experiment 1")
	performExperiment1aFlag := flag.Bool("experiment1a", false, "perform experiment 1a")
	performExperiment1bFlag := flag.Bool("experiment1b", false, "perform experiment 1b")
	performExperiment1cFlag := flag.Bool("experiment1c", false, "perform experiment 1c")
	performExperiment2Flag := flag.Bool("experiment2", false, "perform experiment 2")
	performExperiment2aFlag := flag.Bool("experiment2a", false, "perform experiment 2a")
	performExperiment2bFlag := flag.Bool("experiment2b", false, "perform experiment 2b")
	performExperiment2cFlag := flag.Bool("experiment2c", false, "perform experiment 2c")
	performExperiment2dFlag := flag.Bool("experiment2d", false, "perform experiment 2d")
	performExperiment3Flag := flag.Bool("experiment3", false, "perform experiment 3")
	performExperiment3aFlag := flag.Bool("experiment3a", false, "perform experiment 3a")
	performExperiment3bFlag := flag.Bool("experiment3b", false, "perform experiment 3b")
	performExperiment3cFlag := flag.Bool("experiment3c", false, "perform experiment 3c")
	performExperiment3dFlag := flag.Bool("experiment3d", false, "perform experiment 3d")
	performExperiment3eFlag := flag.Bool("experiment3e", false, "perform experiment 3e")
	performExperiment3fFlag := flag.Bool("experiment3f", false, "perform experiment 3f")
	performExperiment3gFlag := flag.Bool("experiment3g", false, "perform experiment 3g")
	performExperiment3hFlag := flag.Bool("experiment3h", false, "perform experiment 3h")
	performExperiment4Flag := flag.Bool("experiment4", false, "perform experiment 4")
	performExperiment5Flag := flag.Bool("experiment5", false, "perform experiment 5")
	performExperiment6Flag := flag.Bool("experiment6", false, "perform experiment 6")
	performExperiment7Flag := flag.Bool("experiment7", false, "perform experiment 7")
	performExperimentsFlag := flag.Bool("experiments", false, "perform all experiments")
	numUsersFlag := flag.Int("nu", -1, "number of users")
	numDistrictsFlag := flag.Int("nd", -1, "number of districts/subtrees")
	numSamplesFlag := flag.Int("ns", -1, "number of samples")
	flag.Parse()

	if *remote {
		server = network.NewRemoteServer(*serverURL)
		factory = network.NewRemoteFactory(server.(*network.RemoteServer))
	} else {
		server = new(pkg_server.Server)
		factory = new(tables.TableFactory)
	}

	if *numUsersFlag > -1 {
		nUsers = *numUsersFlag
		nUsers6 = []int{*numUsersFlag}
		nUsers7 = []int{*numUsersFlag}
	}

	if *numDistrictsFlag > -1 {
		nDistricts6 = []int{*numDistrictsFlag}
		nDistricts7 = []int{*numDistrictsFlag}
	}

	if *numSamplesFlag > -1 {
		nSamples6 = *numSamplesFlag
	}

	if *createTablesFlag {
		fmt.Printf("creating table\n")
		factory.CreateTableWithRandomMissing("input/table1.txt")
		factory.CreateExample2Table("input/table_example2.txt")
		fmt.Printf("done\n")
		return
	}

	if *performExperiment1Flag {
		runExperiment1a(nEpochs1, nUsers, 1)
		runExperiment1b(nEpochs1, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment1c(nEpochs1, nUsers, nUsers)
	}

	if *performExperiment1aFlag {
		runExperiment1a(nEpochs1, nUsers, 1)
	}

	if *performExperiment1bFlag {
		runExperiment1b(nEpochs1, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment1cFlag {
		runExperiment1c(nEpochs1, nUsers, nUsers)
	}

	if *performExperiment2Flag {
		runExperiment2a(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2b(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2c(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2d(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment2aFlag {
		runExperiment2a(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment2bFlag {
		runExperiment2b(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment2cFlag {
		runExperiment2c(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment2dFlag {
		runExperiment2d(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
	}
	if *performExperiment3Flag {
		runExperiment3a(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3b(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3c(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3d(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3e(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3f(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3g(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3h(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3aFlag {
		runExperiment3a(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3bFlag {
		runExperiment3b(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3cFlag {
		runExperiment3c(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3dFlag {
		runExperiment3d(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3eFlag {
		runExperiment3e(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3fFlag {
		runExperiment3f(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3gFlag {
		runExperiment3g(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment3hFlag {
		runExperiment3h(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment4Flag {
		runExperiment4(nEpochs4, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment5Flag {
		runExperiment5(nEpochs5, nUsers, int(math.Sqrt(float64(nUsers))))
	}

	if *performExperiment6Flag {
		runExperiment6(nEpochs6, nUsers6, nDistricts6, nSamples6)
	}

	if *performExperiment7Flag {
		runExperiment7(nEpochs7, nUsers7, nDistricts7, nSamples7)
	}

	if *performExperimentsFlag {
		runExperiment1a(nEpochs1, nUsers, 1)
		runExperiment1b(nEpochs1, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment1c(nEpochs1, nUsers, nUsers)
		runExperiment2a(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2b(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2c(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment2d(nEpochs2, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3a(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3b(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3c(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3d(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3e(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3f(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3g(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment3h(nEpochs3, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment4(nEpochs4, nUsers, int(math.Sqrt(float64(nUsers))))
		runExperiment5(nEpochs5, nUsers, int(math.Sqrt(float64(nUsers))))
	}
}
