package backup

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestParsePgMajor(t *testing.T) {
	cases := map[string]int{
		"180006":                      18,
		"170011":                      17,
		"pg_dump (PostgreSQL) 18.6":   18,
		"pg_dump (PostgreSQL) 17.0\n": 17,
		"16":                          16,
	}
	for in, want := range cases {
		got, ok := parsePgMajor(in)
		if !ok || got != want {
			t.Fatalf("%q => %d ok=%v, want %d", in, got, ok, want)
		}
	}
}

type pathRunner struct {
	paths map[string]string
}

func (p pathRunner) LookPath(file string) (string, error) {
	if ver, ok := p.paths[file]; ok && ver != "" {
		return file, nil
	}
	return "", fmt.Errorf("not found")
}

func (p pathRunner) Run(_ context.Context, name string, args []string, _ []string) (Result, error) {
	if ver, ok := p.paths[name]; ok {
		return Result{Stdout: ver + "\n"}, nil
	}
	return Result{}, fmt.Errorf("missing %s args=%s", name, strings.Join(args, " "))
}

func TestSelectPgDumpPicksNewestCompatible(t *testing.T) {
	runner := pathRunner{paths: map[string]string{
		"/usr/lib/postgresql/16/bin/pg_dump": "pg_dump (PostgreSQL) 16.4",
		"/usr/lib/postgresql/18/bin/pg_dump": "pg_dump (PostgreSQL) 18.1",
	}}
	bin, ver, err := selectPgDump(t.Context(), runner, 17)
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/lib/postgresql/18/bin/pg_dump" || !strings.Contains(ver, "18.1") {
		t.Fatalf("bin=%s ver=%s", bin, ver)
	}
	_, _, err = selectPgDump(t.Context(), runner, 19)
	if err == nil {
		t.Fatal("expected error for server newer than clients")
	}
}
