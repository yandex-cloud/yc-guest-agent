//go:build linux
// +build linux

package usermanager

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

type (
	nextUsernameFunc func(username string) error
	iterator         func(line string) error
)

func parseByLine(r io.Reader, parseLine iterator) error {
	scanner := bufio.NewScanner(r)

	for {
		if ok := scanner.Scan(); !ok {
			if err := scanner.Err(); err != nil {
				return err
			}

			return nil
		}

		line := scanner.Text()
		if err := parseLine(line); err != nil {
			return err
		}
	}
}

func usernameIterator(fn nextUsernameFunc) iterator {
	return func(line string) error {
		username := parseUsername(line)
		if username != "" {
			return fn(username)
		}

		return nil
	}
}

func parseUsername(line string) string {
	// username:password:UID:GID:info:home:shell
	tokens := strings.SplitN(line, ":", 7)
	if len(tokens) < 7 {
		return ""
	}

	uid, err := strconv.Atoi(tokens[2])
	if err != nil {
		return ""
	}

	// well-known user `nobody`
	if uid == 65534 {
		return ""
	}

	// system user
	if uid < 1000 {
		return ""
	}

	return tokens[0]
}

func parseUsernames(fs afero.Fs, fn nextUsernameFunc) (err error) {
	const usersfile = "/etc/passwd"
	file, err := fs.Open(usersfile)
	if err != nil {
		return
	}
	defer func() {
		clsErr := file.Close()
		if clsErr != nil {
			err = clsErr
		}
	}()

	return parseByLine(file, usernameIterator(fn))
}
