package tables

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"

	"github.com/tap-group/tdsvc/util"
)

const NUM_USERS = 20
const NUM_DISTRICTS = 5
const NUM_TIMESLOTS = 50
const MAX_POWER = 20000
const MISS_FREQ = 4
const INDUSTRY_FREQ = 5

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
	return rand.Intn(math.MaxInt32/2-1) + 1 //rand.Intn(math.MaxInt32-1) + 1 // seed 0 not allowed (p256 does not support this case anyway)
}

func (factory *TableFactory) initialize(nUsers, nDistricts, industryFreq int, mode int) {
	factory.districts = make([]int, nUsers)
	factory.industrial = make([]int, nUsers)

	for i := range factory.districts {
		if mode == RAND_ZERO_TO_ND {
			factory.districts[i] = rand.Intn(nDistricts)
		} else if mode == RAND_ZERO_TO_NU {
			factory.districts[i] = rand.Intn(nUsers)
		} else if mode == RAND_MAX_TO_MAX_MINUS_ND {
			factory.districts[i] = math.MaxInt32 - rand.Intn(nDistricts)
		} else if mode == RAND_ZERO_TO_MAX {
			factory.districts[i] = rand.Intn(math.MaxInt32)
		} else if mode == UNIF_ZERO_TO_ND {
			factory.districts[i] = (i * nDistricts) / nUsers
		} else if mode == UNIF_ZERO_TO_NU {
			factory.districts[i] = i
		} else if mode == UNIF_MAX_TO_MAX_MINUS_ND {
			factory.districts[i] = math.MaxInt32 - (i*nDistricts)/nUsers
		} else if mode == UNIF_ZERO_TO_MAX {
			factory.districts[i] = i * math.MaxInt32 / nUsers
		}
	}
	for i := range factory.industrial {
		industrial := -2
		if industryFreq > 0 {
			industrial = rand.Intn(industryFreq)
		}
		if industrial < industryFreq-1 {
			factory.industrial[i] = 0
		} else {
			factory.industrial[i] = 1
		}
	}
}

func (factory *TableFactory) CreateTableWithoutRandomMissing(filename string) {
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"
	factory.initialize(NUM_USERS, NUM_DISTRICTS, INDUSTRY_FREQ, RAND_ZERO_TO_ND)

	for i := 0; i < NUM_TIMESLOTS; i++ {
		for j := 0; j < NUM_USERS; j++ {
			result += fmt.Sprint(j) + "," + fmt.Sprint(factory.districts[j]) + "," + fmt.Sprint(factory.industrial[j]) + "," + fmt.Sprint(i) + ","
			r := rand.Intn(MAX_POWER) + 1
			if factory.industrial[j] == 1 {
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

func (factory *TableFactory) CreateTableWithRandomMissing(filename string) {
	result := "user,district,is_industrial,timeslot,power,bill,seed\n"
	factory.initialize(NUM_USERS, NUM_DISTRICTS, INDUSTRY_FREQ, RAND_ZERO_TO_ND)

	for i := 0; i < NUM_TIMESLOTS; i++ {
		for j := 0; j < NUM_USERS; j++ {
			miss := rand.Intn(MISS_FREQ)
			if miss < MISS_FREQ-1 {
				result += fmt.Sprint(j) + "," + fmt.Sprint(factory.districts[j]) + "," + fmt.Sprint(factory.industrial[j]) + "," + fmt.Sprint(i) + ","
				r := rand.Intn(MAX_POWER) + 1
				if factory.industrial[j] == 1 {
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

func (factory *TableFactory) CreateTableForExperiment(filename string, nUsers, nDistricts, nTimeslots, startTime, missFreq, industryFreq, maxPower, mode, nonce int) {
	rand.Seed(int64((nUsers + 1) * (nDistricts + 1) * (nTimeslots + 1) * (startTime + 1) * (missFreq + 1) * (maxPower + 1) * (mode + 1) * (nonce + 1)))
	factory.initialize(nUsers, nDistricts, industryFreq, mode)

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	util.Check(err)
	datawriter := bufio.NewWriter(file)
	_, _ = datawriter.WriteString("user,district,is_industrial,timeslot,power,bill,seed\n")

	for i := 0; i < nTimeslots; i++ {
		for j := 0; j < nUsers; j++ {
			miss := -2
			if missFreq > 0 {
				miss = rand.Intn(missFreq)
			}
			if miss < missFreq-1 {
				newRow := fmt.Sprint(j) + "," + fmt.Sprint(factory.districts[j]) + "," + fmt.Sprint(factory.industrial[j]) + "," + fmt.Sprint(i+startTime) + ","
				r := rand.Intn(maxPower) + 1
				if factory.industrial[j] == 1 {
					r *= 10
				}
				newRow += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
				seed := getRandomSeed()
				newRow += fmt.Sprint(seed) + "\n"
				_, _ = datawriter.WriteString(newRow)
			}
		}
	}

	datawriter.Flush()
	file.Close()
}

func (factory *TableFactory) RegenerateTableForExperiment(filename string, nTimeslots, startTime, missFreq, maxPower, mode, nonce int) {
	rand.Seed(int64((nTimeslots + 1) * (startTime + 1) * (missFreq + 1) * (maxPower + 1) * (mode + 1) * (nonce + 1)))
	nUsers := len(factory.districts) // length of array must equal the number of users

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	util.Check(err)
	datawriter := bufio.NewWriter(file)
	_, _ = datawriter.WriteString("user,district,is_industrial,timeslot,power,bill,seed\n")

	for i := 0; i < nTimeslots; i++ {
		for j := 0; j < nUsers; j++ {
			fmt.Println(factory.industrial[j])
			miss := 0.
			if missFreq > 0 {
				miss = 1. / float64(rand.Intn(missFreq))
			}
			if miss < 1 {
				newRow := fmt.Sprint(j) + "," + fmt.Sprint(factory.districts[j]) + "," + fmt.Sprint(factory.industrial[j]) + "," + fmt.Sprint(i+startTime) + ","
				r := rand.Intn(maxPower) + 1
				if factory.industrial[j] == 1 {
					r *= 10
				}
				newRow += fmt.Sprint(r) + "," + fmt.Sprint(2*r) + ","
				seed := getRandomSeed()
				newRow += fmt.Sprint(seed) + "\n"
				_, _ = datawriter.WriteString(newRow)
			}
		}
	}

	datawriter.Flush()
	file.Close()
}

func contains(a []int, value int) bool {
	for _, v := range a {
		if v == value {
			return true
		}
	}
	return false
}

func (factory *TableFactory) CreateExample2Table(filename string) {
	result := "user,timeslot,power,seed\n"
	numTimeslots := 32
	numUsers := 8

	timeslots := []int{2, 3, 5, 7, 10, 12, 13, 14, 16, 18, 31}

	for i := 0; i < numTimeslots; i++ {
		for j := 0; j < numUsers; j++ {
			if contains(timeslots, i) {
				result += fmt.Sprint(j) + "," + fmt.Sprint(i) + ","
				r := rand.Intn(MAX_POWER) + 1
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
