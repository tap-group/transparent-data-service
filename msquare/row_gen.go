package main

import (
	"fmt"
	"math/rand"
)

type Row struct {
	Key       []byte
	Value     []byte
	Signature []byte
}

type RowGenerator struct {
	userCount int

	isIndustry []int
	districts  []int

	epoch int
	index int
}

func NewRowGenerator(userCount, districtCount int) *RowGenerator {
	gen := &RowGenerator{
		userCount:  userCount,
		districts:  make([]int, userCount),
		isIndustry: make([]int, userCount),
	}
	for i := 0; i < userCount; i++ {
		gen.districts[i] = (i * districtCount) / userCount
	}
	for i := 0; i < userCount; i++ {
		gen.isIndustry[i] = rand.Intn(2)
	}
	return gen
}

func (gen *RowGenerator) Reset(epoch int) {
	gen.epoch = epoch
	gen.index = 0
}

func (gen *RowGenerator) HasNext() bool {
	return gen.index < gen.userCount
}

func (gen *RowGenerator) Next() Row {
	row := gen.MakeRow(gen.index, gen.epoch)
	gen.index++
	return row
}

func (gen *RowGenerator) MakeRow(userIndex, epoch int) Row {
	district, isIndustry := gen.districts[userIndex], gen.isIndustry[userIndex]
	return Row{
		Key:       []byte(fmt.Sprintf("%d_%d_%d", district, isIndustry, epoch)),
		Value:     []byte(fmt.Sprintf("%d_%d", userIndex, epoch*2000)),
		Signature: []byte(fmt.Sprintf("%d_%d", userIndex, epoch)),
	}
}
