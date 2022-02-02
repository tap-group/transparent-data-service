package main

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"text/tabwriter"
	"time"

	"github.com/tap-group/tdsvc/util"
	"github.com/ucbrise/MerkleSquare/core"
)

var (
	UserCount     = 100
	DistrictCount = 10
	RunCount      = 5
)

type Cost struct {
	Storage        int
	Insert         time.Duration
	UserMonitor    time.Duration
	AuditorMonitor time.Duration
}

func main() {
	costs := make([]Cost, 0)
	costs = append(costs, runExperimentsAverage(1, RunCount))
	costs = append(costs, runExperimentsAverage(2, RunCount))
	costs = append(costs, runExperimentsAverage(5, RunCount))
	costs = append(costs, runExperimentsAverage(10, RunCount))
	costs = append(costs, runExperimentsAverage(20, RunCount))
	costs = append(costs, runExperimentsAverage(40, RunCount))
	costs = append(costs, runExperimentsAverage(80, RunCount))
	costs = append(costs, runExperimentsAverage(160, RunCount))

	fmt.Print("\n\n")
	f, err := os.Create("output.txt")
	util.Check(err)
	defer f.Close()
	writeCosts(f, costs)
	// measureStorageCost(1)
	// measureStorageCost(10)
	// measureStorageCost(100)
}

// func measureStorageCost(epochs int) {
// 	rowGen := NewRowGenerator(UserCount, DistrictCount)
// 	m := createTree(rowGen, epochs)
// 	fmt.Println("size: ", m.GetMerkleSquareSize())
// }

func writeCosts(bw io.Writer, costs []Cost) {
	w := tabwriter.NewWriter(bw, 0, 0, 5, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "Storage\tInsert\tUser\tAuditor\t")
	for _, cost := range costs {
		fmt.Fprintf(w, "%v\t%.6f\t%.6f\t%.6f\t\n",
			cost.Storage,
			cost.Insert.Seconds(),
			cost.UserMonitor.Seconds(),
			cost.AuditorMonitor.Seconds(),
		)
	}
	w.Flush()
}

func runExperimentsAverage(epochs, count int) Cost {
	average := Cost{}
	for i := 0; i < count; i++ {
		cost := runExperiments(epochs)
		average.Storage += cost.Storage
		average.Insert += cost.Insert
		average.UserMonitor += cost.UserMonitor
		average.AuditorMonitor += cost.AuditorMonitor
	}
	average.Storage /= count
	average.Insert /= time.Duration(count)
	average.UserMonitor /= time.Duration(count)
	average.AuditorMonitor /= time.Duration(count)
	return average
}

func runExperiments(epochs int) Cost {
	fmt.Printf("running experiment, epochs=%d\n", epochs)
	rowGen := NewRowGenerator(UserCount, DistrictCount)
	mSquare := createTree(rowGen, epochs)

	cost := Cost{}
	cost.Storage = mSquare.GetMerkleSquareSize()

	var start time.Time
	start = time.Now()
	insertOneEpoch(mSquare, rowGen, epochs)
	cost.Insert = time.Since(start)

	start = time.Now()
	runLoopUpProof(mSquare, rowGen)
	cost.UserMonitor = time.Since(start)

	start = time.Now()
	runExtensionProof(mSquare)
	cost.AuditorMonitor = time.Since(start)

	return cost
}

func createTree(rowGen *RowGenerator, epochs int) *core.MerkleSquare {
	depth := uint32(math.Log2(float64(UserCount*epochs+UserCount))) + 2
	mSquare := core.NewMerkleSquare(depth)
	for e := 0; e < epochs; e++ {
		insertOneEpoch(mSquare, rowGen, e)
	}
	return mSquare
}

func insertOneEpoch(mSquare *core.MerkleSquare, rowGen *RowGenerator, epoch int) {
	rowGen.Reset(epoch)
	for rowGen.HasNext() {
		row := rowGen.Next()
		// fmt.Printf("Row: %s %s %s\n", row.Key, row.Value, row.Signature)
		mSquare.Append(row.Key, row.Value, row.Signature)
	}
}

func runLoopUpProof(mSquare *core.MerkleSquare, rowGen *RowGenerator) {
	pos := rand.Intn(int(mSquare.Size))
	userIndex := pos % UserCount
	epoch := pos / UserCount
	row := rowGen.MakeRow(userIndex, epoch)

	var res bool
	pkProof := mSquare.ProveLatest(row.Key, row.Value, uint32(pos), mSquare.Size)
	res = core.VerifyPKProof(
		mSquare.GetDigest(), row.Key, row.Value, row.Signature, uint32(pos), pkProof)
	_ = res
}

func runExtensionProof(mSquare *core.MerkleSquare) {
	oldSize := mSquare.Size - uint32(UserCount)
	newSize := mSquare.Size

	oldDigest := mSquare.GetOldDigest(oldSize)
	newDigest := mSquare.GetOldDigest(newSize)

	proof := mSquare.GenerateExtensionProof(oldSize, newSize)

	res := core.VerifyExtensionProof(oldDigest, newDigest, proof)
	_ = res
	// fmt.Println("extension proof: ", res)
}
