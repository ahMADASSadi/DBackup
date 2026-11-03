package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"dbackup/internal/security"
	"golang.org/x/sys/unix"
)

func decrypt(args []string) int {
	flags := flag.NewFlagSet("decrypt", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	in := flags.String("in", "", "encrypted backup")
	out := flags.String("out", "", "restored dump")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: dbackup decrypt -in backup.dbk -out backup.dump")
		return 2
	}
	password, err := readPassword()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read password")
		return 1
	}
	if err := security.OpenFile(*out, *in, password); err != nil {
		fmt.Fprintln(os.Stderr, "decrypt failed")
		return 1
	}
	return 0
}

func readPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
	fd := int(os.Stdin.Fd())
	state, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		line, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", readErr
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	hidden := *state
	hidden.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &hidden); err != nil {
		return "", err
	}
	defer func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, state)
		fmt.Fprintln(os.Stderr)
	}()
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
