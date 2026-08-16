package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runScript(t *testing.T, commands []string) string {
	return runScriptWithDatabase(
		t,
		filepath.Join(t.TempDir(), "test.db"),
		commands,
	)
}

func runScriptWithDatabase(t *testing.T, databaseFilename string, commands []string) string {
	t.Helper()

	cmd := exec.Command("go", "run", ".", databaseFilename)
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
	commands := make([]string, 0, leafNodeMaxCells+2)

	for i := 1; i <= leafNodeMaxCells+1; i++ {
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

func TestKeepsDataAfterClosingConnection(t *testing.T) {
	databaseFilename := filepath.Join(t.TempDir(), "test.db")

	firstOutput := runScriptWithDatabase(t, databaseFilename, []string{
		"insert 1 user1 person1@example.com",
		".exit",
	})
	firstWant := "db > Executed.\ndb > "

	if firstOutput != firstWant {
		t.Errorf(
			"first output mismatch\nwant:\n%q\ngot:\n%q",
			firstWant,
			firstOutput,
		)
	}

	secondOutput := runScriptWithDatabase(t, databaseFilename, []string{
		"select",
		".exit",
	})
	secondWant := "db > (1, user1, person1@example.com)\n" +
		"Executed.\n" +
		"db > "

	if secondOutput != secondWant {
		t.Errorf(
			"second outout mismatch\nwant:\n%qgot:\n%q",
			secondWant,
			secondOutput,
		)
	}
}

func TestPrintConstants(t *testing.T) {
	got := runScript(t, []string{
		".constants",
		".exit",
	})

	want := "db > Constants:\n" +
		"ROW_SIZE: 291\n" +
		"COMMON_NODE_HEADER_SIZE: 6\n" +
		"LEAF_NODE_HEADER_SIZE: 10\n" +
		"LEAF_NODE_CELL_SIZE: 295\n" +
		"LEAF_NODE_SPACE_FOR_CELLS: 4086\n" +
		"LEAF_NODE_MAX_CELLS: 13\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestPrintOneNodeBTree(t *testing.T) {
	got := runScript(t, []string{
		"insert 3 user3 person3@example.com",
		"insert 1 user1 person1@example.com",
		"insert 2 user2 person2@example.com",
		".btree",
		".exit",
	})

	want := "db > Executed.\n" +
		"db > Executed.\n" +
		"db > Executed.\n" +
		"db > Tree:\n" +
		"leaf (size 3)\n" +
		"  - 0 : 3\n" +
		"  - 1 : 1\n" +
		"  - 2 : 2\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}
