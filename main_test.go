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
		"LEAF_NODE_HEADER_SIZE: 14\n" +
		"LEAF_NODE_CELL_SIZE: 295\n" +
		"LEAF_NODE_SPACE_FOR_CELLS: 4082\n" +
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
		"- leaf (size 3)\n" +
		"  - 1\n" +
		"  - 2\n" +
		"  - 3\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestDuplicateKeys(t *testing.T) {
	got := runScript(t, []string{
		"insert 1 user1 person1@example.com",
		"insert 1 user1 person1@example.com",
		"select",
		".exit",
	})

	want := "db > Executed.\n" +
		"db > Error: Duplicate key.\n" +
		"db > (1, user1, person1@example.com)\n" +
		"Executed.\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestPrintThreeLeafNodeBTree(t *testing.T) {
	commands := make([]string, 0, leafNodeMaxCells+2)

	for i := 1; i <= leafNodeMaxCells+1; i++ {
		commands = append(
			commands,
			fmt.Sprintf("insert %d user%d person%d@example.com", i, i, i),
		)
	}

	commands = append(
		commands,
		".btree",
		"insert 15 user15 person15@example.com",
		".exit",
	)

	got := runScript(t, commands)

	want := strings.Repeat(
		"db > Executed.\n",
		int(leafNodeMaxCells+1),
	) +
		"db > Tree:\n" +
		"- internal (size 1)\n" +
		"  - leaf (size 7)\n" +
		"    - 1\n" +
		"    - 2\n" +
		"    - 3\n" +
		"    - 4\n" +
		"    - 5\n" +
		"    - 6\n" +
		"    - 7\n" +
		"  - key 7\n" +
		"  - leaf (size 7)\n" +
		"    - 8\n" +
		"    - 9\n" +
		"    - 10\n" +
		"    - 11\n" +
		"    - 12\n" +
		"    - 13\n" +
		"    - 14\n" +
		"db > Executed.\n" +
		"db > "

	if got != want {
		t.Errorf("output mismatch\nwant\n%q\ngot:\n%q", want, got)
	}
}

func TestSelectFromMultiLevelBTree(t *testing.T) {
	commands := make([]string, 0, leafNodeMaxCells+4)

	for i := 1; i <= leafNodeMaxCells+2; i++ {
		commands = append(
			commands,
			fmt.Sprintf("insert %d user%d person%d@example.com", i, i, i),
		)
	}

	commands = append(commands, "select", ".exit")

	got := runScript(t, commands)
	var want strings.Builder

	for i := 1; i <= leafNodeMaxCells+2; i++ {
		want.WriteString("db > Executed.\n")
	}

	want.WriteString("db > ")

	for i := 1; i <= leafNodeMaxCells+2; i++ {
		fmt.Fprintf(
			&want,
			"(%d, user%d, person%d@example.com)\n", i, i, i,
		)
	}

	want.WriteString("Executed.\n")
	want.WriteString("db > ")

	if got != want.String() {
		t.Errorf(
			"output mismatch\nwant:\n%q\ngot:\n%q",
			want.String(),
			got,
		)
	}
}

func TestPrintFourLeafNodeBTree(t *testing.T) {
	ids := []int{
		18, 7, 10, 29, 23, 4, 14, 30, 15, 26,
		22, 19, 2, 1, 21, 11, 6, 20, 5, 8,
		9, 3, 12, 27, 17, 16, 13, 24, 25, 28,
	}

	commands := make([]string, 0, len(ids)+2)

	for _, id := range ids {
		commands = append(
			commands,
			fmt.Sprintf(
				"insert %d user%d person%d@example.com",
				id,
				id,
				id,
			),
		)
	}

	commands = append(commands, ".btree", ".exit")

	got := runScript(t, commands)

	wantSuffix := "db > Tree:\n" +
		"- internal (size 3)\n" +
		"  - leaf (size 7)\n" +
		"    - 1\n" +
		"    - 2\n" +
		"    - 3\n" +
		"    - 4\n" +
		"    - 5\n" +
		"    - 6\n" +
		"    - 7\n" +
		"  - key 7\n" +
		"  - leaf (size 8)\n" +
		"    - 8\n" +
		"    - 9\n" +
		"    - 10\n" +
		"    - 11\n" +
		"    - 12\n" +
		"    - 13\n" +
		"    - 14\n" +
		"    - 15\n" +
		"  - key 15\n" +
		"  - leaf (size 7)\n" +
		"    - 16\n" +
		"    - 17\n" +
		"    - 18\n" +
		"    - 19\n" +
		"    - 20\n" +
		"    - 21\n" +
		"    - 22\n" +
		"  - key 22\n" +
		"  - leaf (size 8)\n" +
		"    - 23\n" +
		"    - 24\n" +
		"    - 25\n" +
		"    - 26\n" +
		"    - 27\n" +
		"    - 28\n" +
		"    - 29\n" +
		"    - 30\n" +
		"db > "

	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf(
			"tree output mismatch\nwant suffix:\n%q\ngot:\n%q",
			wantSuffix,
			got,
		)
	}
}
