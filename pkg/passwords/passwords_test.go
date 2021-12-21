package passwords

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerator(t *testing.T) {
	g := NewGenerator("", "", "", "")

	t.Run("catch no err", func(t *testing.T) {
		_, err := g.Generate(15, 3, 2, false)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("catch errLengthTooShort", func(t *testing.T) {
		_, err := g.Generate(15, 16, 17, false)
		if !errors.Is(err, ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = g.Generate(0, 16, 17, false)
		if !errors.Is(err, ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = g.Generate(0, 0, 17, false)
		if !errors.Is(err, ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = g.Generate(0, 16, 0, false)
		if !errors.Is(err, ErrLengthTooShort) {
			t.Error(err)
		}
	})

	t.Run("number of digits", func(t *testing.T) {
		// correct number of digits
		pwd, err := g.Generate(15, 3, 2, false)
		if err != nil {
			t.Error(err)
		}
		gotDigits := 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultDigits, r) {
				gotDigits++
			}
		}
		if gotDigits != 3 {
			t.Errorf("expected: %v, got: %v digits in password: %v", 3, gotDigits, pwd)
		}

		// only digits
		pwd, err = g.Generate(15, 15, 0, false)
		if err != nil {
			t.Error(err)
		}
		gotDigits = 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultDigits, r) {
				gotDigits++
			}
		}
		if gotDigits != 15 {
			t.Errorf("expected: %v, got: %v digits in password: %v", 3, gotDigits, pwd)
		}

		// no digits
		pwd, err = g.Generate(15, 0, 0, false)
		if err != nil {
			t.Errorf("expected no errors, got: %v", err)
		}

		gotDigits = 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultDigits, r) {
				gotDigits++
			}
		}
		if gotDigits != 0 {
			t.Errorf("expected: %v, got: %v digits in password: %v", 3, gotDigits, pwd)
		}
	})

	t.Run("number of symbols", func(t *testing.T) {
		// correct number of symbols
		pwd, err := g.Generate(15, 3, 2, false)
		if err != nil {
			t.Error(err)
		}
		gotSymbols := 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultSymbols, r) {
				gotSymbols++
			}
		}
		if gotSymbols != 2 {
			t.Errorf("expected: %v, got: %v symbols in password: %v", 2, gotSymbols, pwd)
		}

		// only symbols
		pwd, err = g.Generate(15, 0, 15, false)
		if err != nil {
			t.Error(err)
		}
		gotSymbols = 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultSymbols, r) {
				gotSymbols++
			}
		}
		if gotSymbols != 15 {
			t.Errorf("expected: %v, got: %v symbols in password: %v", 2, gotSymbols, pwd)
		}

		// no symbols
		pwd, err = g.Generate(15, 3, 0, false)
		if err != nil {
			t.Error(err)
		}
		gotSymbols = 0
		for _, r := range pwd {
			if strings.ContainsRune(defaultSymbols, r) {
				gotSymbols++
			}
		}
		if gotSymbols != 0 {
			t.Errorf("expected: %v, got: %v symbols in password: %v", 2, gotSymbols, pwd)
		}
	})

	t.Run("no uppers", func(t *testing.T) {
		// literally no uppers
		pwd, err := g.Generate(15, 3, 2, true)
		if err != nil {
			t.Error(err)
		}
		gotUpperLetters := 0
		for _, r := range pwd {
			// defaultUpperLetters better, but couldn't be accessed, implement method?
			if strings.ContainsRune(defaultUpperLetters, r) {
				gotUpperLetters++
			}
		}
		if gotUpperLetters > 0 {
			t.Errorf("expected no upper letters, got: %v in password: %v", gotUpperLetters, pwd)
		}

		// only lowers
		pwd, err = g.Generate(15, 0, 0, true)
		if err != nil {
			t.Error(err)
		}
		if strings.ToLower(pwd) != pwd {
			t.Error(err)
		}
	})

	t.Run("entropy small check", func(t *testing.T) {
		passwords := make(map[string]int, 100000)
		for i := 0; i < 100000; i++ {
			pwd, err := g.Generate(15, 3, 2, false)
			if err != nil {
				t.Error(err)
			}

			if prev, ok := passwords[pwd]; !ok {
				passwords[pwd] = i
			} else {
				t.Errorf("got duplicated password: %v on %v and %v iteration", pwd, prev, i)
			}
		}
	})
}
