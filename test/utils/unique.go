package utils

import (
	"bytes"
	"math/rand"
	"time"
)

const base36chars = "0123456789abcdefghijklmnopqrstuvwxyz"
const uniqueIDLength = 7 // Should be good for 36^7 = 78+ billion combinations

func UniqueId() string {
	var out bytes.Buffer

	generator := newRand()
	for i := 0; i < uniqueIDLength; i++ {
		out.WriteByte(base36chars[generator.Intn(len(base36chars))])
	}

	return out.String()
}

// newRand creates a new random number generator, seeding it with the current system time.
func newRand() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}
