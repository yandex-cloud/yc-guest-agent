package passwords

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	mathRand "math/rand"
)

const (
	defaultLowerLetters = "abcdefghijklmnopqrstuvwxyz"
	defaultUpperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	defaultDigits       = "0123456789"
	defaultSymbols      = `~!@#$%^&*_-+=|(){}[]:;<>,.?`
)

var ErrLengthTooShort = errors.New("number of symbols and digits is lover than length")

type Generator struct {
	lowerLetters string
	upperLetters string
	digits       string
	symbols      string
}

type GeneratorInterface interface {
	Generate(length, numDigits, numSymbols uint, noUpper bool) (string, error)
}

func NewGenerator(lowerLetters, upperLetters, digits, symbols string) GeneratorInterface {
	g := &Generator{
		lowerLetters: defaultLowerLetters,
		upperLetters: defaultUpperLetters,
		digits:       defaultDigits,
		symbols:      defaultSymbols,
	}

	if lowerLetters != "" {
		g.lowerLetters = lowerLetters
	}

	if upperLetters != "" {
		g.upperLetters = upperLetters
	}

	if digits != "" {
		g.digits = digits
	}

	if symbols != "" {
		g.symbols = symbols
	}

	return g
}

func (g *Generator) Generate(length, numDigits, numSymbols uint, noUpper bool) (pwd string, err error) {
	letters := g.lowerLetters
	if !noUpper {
		letters += g.upperLetters
	}

	if length < numDigits+numSymbols {
		err = ErrLengthTooShort
		return
	}

	// digits
	for i := uint(0); i < numDigits; i++ {
		pwd += getRandom(g.digits)
	}

	// symbols
	for i := uint(0); i < numSymbols; i++ {
		pwd += getRandom(g.symbols)
	}

	// letters
	for i := uint(0); i < length-(numDigits+numSymbols); i++ {
		pwd += getRandom(letters)
	}

	// shuffle
	pwd = shuffle(pwd)
	return
}

// https://stackoverflow.com/questions/35203635/golang-cryptographic-shuffle
type cryptoRandSource struct{}

func newCryptoRandSource() cryptoRandSource {
	return cryptoRandSource{}
}

//nolint:ST1006
func (_ cryptoRandSource) Int63() int64 {
	var b [8]byte
	//nolint:ST1006
	_, err := rand.Read(b[:])
	if err != nil {
		panic("could not allocate random buffer")
	}
	// mask off sign bit to ensure positive number
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<63 - 1))
}

//nolint:ST1006
func (_ cryptoRandSource) Seed(_ int64) {}

func getRandom(s string) string {
	r := mathRand.New(newCryptoRandSource())
	n := r.Int63n(int64(len(s)))

	return string(s[n])
}

func shuffle(s string) string {
	r := mathRand.New(newCryptoRandSource())
	runes := []rune(s)
	r.Shuffle(len(s),
		func(i, j int) {
			runes[i], runes[j] = runes[j], runes[i]
		})

	return string(runes)
}
