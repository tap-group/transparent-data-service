package tables

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"

	"github.com/tap-group/tdsvc/util"
)

const NUM_USERS = 20
const NUM_DISTRICTS = 5
const NUM_TIMESLOTS = 50
const MAX_POWER = 20000
const MISS_FREQ = 4

const RAND_ZERO_TO_ND = 0
const RAND_ZERO_TO_NU = 1
const RAND_MAX_TO_MAX_MINUS_ND = 2
const RAND_ZERO_TO_MAX = 3
const UNIF_ZERO_TO_ND = 4
const UNIF_ZERO_TO_NU = 5
const UNIF_MAX_TO_MAX_MINUS_ND = 6
const UNIF_ZERO_TO_MAX = 7

type TableFactory struct {
	districts  []int
	industrial []int
}

func getRandomSeed() int {
	return rand.Intn(math.MaxInt32-1) + 1 //rand.Intn(math.MaxInt32-1) + 1 // seed 0 not allowed (p256 does not support this case anyway)
}

func (table *TableFactory) initialize(nUsers, nDistricts, mode int) {
	table.districts = make([]int, nUsers)
	table.industrial = make([]int, nUsers)

	for i := range table.districts {
		if mode == RAND_ZERO_TO_ND {
			table.districts[i] = rand.Intn(nDistricts)
		} else if mode == RAND_ZERO_TO_NU {
			table.districts[i] = rand.Intn(nUsers)
		} else if mode == RAND_MAX_TO_MAX_MINUS_ND {
			table.districts[i] = math.MaxInt32 - rand.Intn(nDistricts)
		} else if mode == RAND_ZERO_TO_MAX {
			table.districts[i] = rand.Intn(math.MaxInt32)
		} else if mode == UNIF_ZERO_TO_ND {
			table.districts[i] = (i * nDistricts) / nUsers
		} else if mode == UNIF_ZERO_TO_NU {
			table.districts[i] = i
		} else if mode == UNIF_MAX_TO_MAX_MINUS_ND {
			table.districts[i] = math.MaxInt32 - (i*nDistricts)/nUsers
		} else if mode == UNIF_ZERO_TO_MAX {
			table.districts[i] = i * math.MaxInt32 / nUsers
		}
	}
	for i := range table.industrial {
		rv := rand.Intn(5)
		if rv == 0 {
			table.industrial[i] = 1
		} else {
			table.industrial[i] = 0
		}
	}
}

func (table *TableFactory) CreateTableWithoutRandomMissing(filename string) {
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"
	table.initialize(NUM_USERS, NUM_DISTRICTS, RAND_ZERO_TO_ND)

	for i := 0; i < NUM_TIMESLOTS; i++ {
		for j := 0; j < NUM_USERS; j++ {
			result += fmt.Sprint(j) + "," + fmt.Sprint(table.districts[j]) + "," + fmt.Sprint(table.industrial[j]) + "," + fmt.Sprint(i) + ","
			r := rand.Intn(MAX_POWER)
			if table.industrial[j] == 1 {
				r *= 10
			}
			result += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
			seed := getRandomSeed()
			result += fmt.Sprint(seed) + "\n"
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}

func (table *TableFactory) CreateTableWithRandomMissing(filename string) {
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"
	table.initialize(NUM_USERS, NUM_DISTRICTS, RAND_ZERO_TO_ND)

	for i := 0; i < NUM_TIMESLOTS; i++ {
		for j := 0; j < NUM_USERS; j++ {
			miss := rand.Intn(MISS_FREQ)
			if miss < MISS_FREQ-1 {
				result += fmt.Sprint(j) + "," + fmt.Sprint(table.districts[j]) + "," + fmt.Sprint(table.industrial[j]) + "," + fmt.Sprint(i) + ","
				r := rand.Intn(MAX_POWER)
				if table.industrial[j] == 1 {
					r *= 10
				}
				result += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
				seed := getRandomSeed()
				result += fmt.Sprint(seed) + "\n"
			}
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}

func (table *TableFactory) CreateTableWithRandomMissingForExperiment(filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, maxPower, mode int) {
	rand.Seed(int64((nUsers + 1) * (nDistricts + 1) * (nTimeslots + 1) * (startTime + 1) * (missFreq + 1) * (maxPower + 1) * (mode + 1)))
	table.initialize(nUsers, nDistricts, mode)
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"

	for i := 0; i < nTimeslots; i++ {
		for j := 0; j < nUsers; j++ {
			// miss := rand.Intn(missFreq)
			// if miss < missFreq-1 {
			result += fmt.Sprint(j) + "," + fmt.Sprint(table.districts[j]) + "," + fmt.Sprint(table.industrial[j]) + "," + fmt.Sprint(i+startTime) + ","
			r := rand.Intn(maxPower)
			if table.industrial[j] == 1 {
				r *= 10
			}
			result += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
			seed := getRandomSeed()
			result += fmt.Sprint(seed) + "\n"
			// }
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}

func (table *TableFactory) RegenerateTableWithRandomMissingForExperiment(filename string, nTimeslots, startTime, missFreq, maxPower, mode int) {
	rand.Seed(int64((nTimeslots + 1) * (startTime + 1) * (missFreq + 1) * (maxPower + 1) * (mode + 1)))
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"

	for i := 0; i < nTimeslots; i++ {
		for j := 0; j < len(table.districts); j++ {
			// miss := rand.Intn(missFreq)
			// if miss < missFreq-1 {
			result += fmt.Sprint(j) + "," + fmt.Sprint(table.districts[j]) + "," + fmt.Sprint(table.industrial[j]) + "," + fmt.Sprint(i+startTime) + ","
			r := rand.Intn(maxPower)
			if table.industrial[j] == 1 {
				r *= 10
			}
			result += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
			seed := getRandomSeed()
			result += fmt.Sprint(seed) + "\n"
			// }
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}

func (table *TableFactory) RegenerateTableWithoutRandomMissingForExperiment(filename string, nTimeslots, startTime, maxPower, mode int) {
	rand.Seed(int64((nTimeslots + 1) * (startTime + 1) * (maxPower + 1) * (mode + 1)))
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"

	for i := 0; i < nTimeslots; i++ {
		for j := 0; j < len(table.districts); j++ {
			result += fmt.Sprint(j) + "," + fmt.Sprint(table.districts[j]) + "," + fmt.Sprint(table.industrial[j]) + "," + fmt.Sprint(i+startTime) + ","
			r := rand.Intn(maxPower)
			if table.industrial[j] == 1 {
				r *= 10
			}
			result += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
			seed := getRandomSeed()
			result += fmt.Sprint(seed) + "\n"
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}

func contains(a []int, value int) bool {
	for _, v := range a {
		if v == value {
			return true
		}
	}
	return false
}

func (table *TableFactory) CreateExample2Table(filename string) {
	result := "user,timeslot,power,seed\n"
	numTimeslots := 32
	numUsers := 8

	timeslots := []int{2, 3, 5, 7, 10, 12, 13, 14, 16, 18, 31}

	for i := 0; i < numTimeslots; i++ {
		for j := 0; j < numUsers; j++ {
			if contains(timeslots, i) {
				result += fmt.Sprint(j) + "," + fmt.Sprint(i) + ","
				r := rand.Intn(MAX_POWER)
				result += fmt.Sprint(r) + ","
				seed := getRandomSeed()
				result += fmt.Sprint(seed) + "\n"
			}
		}
	}

	d1 := []byte(result)
	err := ioutil.WriteFile(filename, d1, 0644)
	util.Check(err)
}
