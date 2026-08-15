package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func runScript(t *testing.T, commands []string) string {
	t.Helper()

	cmd := exec.Command("go", "run", ".")
	cmd.Stdin = strings.NewReader(strings.Join(commands, "\n") + "\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("database process failed: %v\n%s", err, output)
	}

	return string(output)
}

func TestInsertAndSelect(t *testing.T) {
	got := runScript(t, []string{
		"insert 1 user1 person1@example.com",
		"select",
		".exit",
	})

	want := "" +
		"db > Executed.\n" +
		"db > (1, user1, person1@example.com)\n" +
		"Executed.\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestTableFull(t *testing.T) {
	commands := make([]string, 0, tableMaxRows+2)

	for i := 1; i <= tableMaxRows+1; i++ {
		commands = append(
			commands,
			fmt.Sprintf("insert %d user%d person%d@example.com", i, i, i),
		)
	}

	commands = append(commands, ".exit")

	got := runScript(t, commands)
	wantSuffix := "db > Error: Table full.\ndb > "

	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("expected table-full error at the end\ngot:\n%q", got)
	}
}

func TestMaximumLengthStrings(t *testing.T) {
	username := strings.Repeat("a", columnUsernameSize)
	email := strings.Repeat("a", columnEmailSize)

	got := runScript(t, []string{
		fmt.Sprintf("insert 1 %s %s", username, email),
		"select", ".exit",
	})

	want := fmt.Sprintf(
		"db > Executed.\n"+
			"db > (1, %s, %s)\n"+
			"Executed.\n"+
			"db > ", username, email,
	)

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestStringTooLong(t *testing.T) {
	username := strings.Repeat("a", columnUsernameSize+1)
	email := strings.Repeat("a", columnEmailSize+1)

	got := runScript(t, []string{
		fmt.Sprintf("insert 1 %s %s", username, email),
		"select",
		".exit",
	})

	want := "db > String is too long.\n" +
		"db > Executed.\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestNagativeId(t *testing.T) {
	got := runScript(t, []string{
		"insert -1 cstack foo@bar.com",
		"select",
		".exit",
	})

	want := "db > ID must be positive.\n" +
		"db > Executed.\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}
