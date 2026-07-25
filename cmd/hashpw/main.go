// Command hashpw prints a bcrypt hash (cost 12) of a password from argv or stdin.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
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
	if len(os.Args) > 1 {
		return os.Args[1], nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}
