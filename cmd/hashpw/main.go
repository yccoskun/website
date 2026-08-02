// Command hashpw prints a bcrypt hash (cost 12) of a password from an
// interactive prompt or piped stdin. Command-line arguments are rejected so
// the password never appears in process listings or shell history.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	password, err := readPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashpw: %v\n", err)
		os.Exit(1)
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "hashpw: empty password")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashpw: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}

func readPassword() (string, error) {
	return readPasswordInput(os.Args[1:], os.Stdin, int(os.Stdin.Fd()), os.Stderr, term.IsTerminal, term.ReadPassword)
}

func readPasswordInput(
	args []string,
	stdin io.Reader,
	fd int,
	stderr io.Writer,
	isTerm func(int) bool,
	readTerm func(int) ([]byte, error),
) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("use interactive prompt or pipe stdin; do not pass the password on the command line")
	}
	if isTerm(fd) {
		fmt.Fprint(stderr, "Password: ")
		b, err := readTerm(fd)
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		// Interactive input is used as-is (no \r\n trim); only the pipe path trims.
		return string(b), nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}
