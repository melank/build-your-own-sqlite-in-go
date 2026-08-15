package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type InputBuffer struct {
	buffer      string
	inputLength int
}

type MetaCommandResult int

const (
	MetaCommandSuccess MetaCommandResult = iota
	MetaCommandUnrecognizedCommand
)

type PrepareResult int

const (
	PrepareSuccess PrepareResult = iota
	PrepareSyntaxError
	PrepareUnrecognizedStatement
)

type StatementType int

const (
	StatementInsert StatementType = iota
	StatementSelect
)

const (
	columnUsernameSize = 32
	columnEmailSize    = 255

	idSize       = 4
	usernameSize = columnUsernameSize
	emailSize    = columnEmailSize

	idOffset       = 0
	usernameOffset = idOffset + idSize
	emailOffset    = usernameOffset + usernameSize
	rowSize        = idSize + usernameSize + emailSize

	pageSize      = 4096
	tableMaxPages = 100
	rowsPerPage   = pageSize / rowSize
	tableMaxRows  = rowsPerPage * tableMaxPages
)

type Row struct {
	id       uint32
	username string
	email    string
}

type ExecuteResult int

const (
	ExecuteSuccess ExecuteResult = iota
	ExecuteTableFull
)

type Table struct {
	numRows uint32
	pages   [tableMaxPages][]byte
}

type Statement struct {
	statementType StatementType
	rowToInsert   Row
}

func doMetaCommand(inputBuffer *InputBuffer) MetaCommandResult {
	if inputBuffer.buffer == ".exit" {
		os.Exit(0)
	}

	return MetaCommandUnrecognizedCommand
}

func prepareStatement(
	inputBuffer *InputBuffer,
	statement *Statement,
) PrepareResult {
	if strings.HasPrefix(inputBuffer.buffer, "insert") {
		statement.statementType = StatementInsert

		fields := strings.Fields(inputBuffer.buffer)
		if len(fields) < 4 {
			return PrepareSyntaxError
		}

		id, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return PrepareSyntaxError
		}

		statement.rowToInsert.id = uint32(id)
		statement.rowToInsert.username = fields[2]
		statement.rowToInsert.email = fields[3]

		return PrepareSuccess
	}

	if inputBuffer.buffer == "select" {
		statement.statementType = StatementSelect
		return PrepareSuccess
	}

	return PrepareUnrecognizedStatement
}

func printRow(row *Row) {
	fmt.Printf("(%d, %s, %s)\n", row.id, row.username, row.email)
}

func serializeRow(source *Row, destination []byte) {
	binary.LittleEndian.PutUint32(
		destination[idOffset : idOffset + idSize],
		source.id,
	)
	copy(
		destination[usernameOffset : usernameOffset + usernameSize],
		source.username,
	)
	copy(
		destination[emailOffset : emailOffset + emailSize],
		source.email,
	)
}

func deserializeRow(source []byte, destination *Row) {
	destination.id = binary.LittleEndian.Uint32(
		source[idOffset : idOffset+idSize],
	)
	destination.username = strings.TrimRight(
		string(source[usernameOffset:usernameOffset+usernameSize]),
		"\x00",
	)
	destination.email = strings.TrimRight(
		string(source[emailOffset:emailOffset+emailSize]),
		"\x00",
	)
}

func rowSlot(table *Table, rowNum uint32) []byte {
	pageNum := rowNum / rowsPerPage
	page := table.pages[pageNum]

	if page == nil {
		page = make([]byte, pageSize)
		table.pages[pageNum] = page
	}

	rowOffset := rowNum % rowsPerPage
	byteOffset := rowOffset * rowSize

	return page[byteOffset : byteOffset+rowSize]
}

func newTable() *Table {
	return &Table{}
}

func executeStatement(statement *Statement) {
	switch statement.statementType {
	case StatementInsert:
		fmt.Println("This is where we would do an insert.")
	case StatementSelect:
		fmt.Println("This is where we would do a select.")
	}
}

func printPrompt() {
	fmt.Print("db > ")
}

func readInput(inputBuffer *InputBuffer, reader *bufio.Reader) error {
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	inputBuffer.buffer = strings.TrimSuffix(input, "\n")
	inputBuffer.inputLength = len(inputBuffer.buffer)

	return nil
}

func main() {
	inputBuffer := &InputBuffer{}
	reader := bufio.NewReader(os.Stdin)

	for {
		printPrompt()

		if err := readInput(inputBuffer, reader); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input")
			return
		}

		if strings.HasPrefix(inputBuffer.buffer, ".") {
			switch doMetaCommand(inputBuffer) {
			case MetaCommandSuccess:
				continue
			case MetaCommandUnrecognizedCommand:
				fmt.Printf("Unrecognized command '%s'\n", inputBuffer.buffer)
				continue
			}
		}

		var statement Statement
		switch prepareStatement(inputBuffer, &statement) {
		case PrepareSuccess:
			// 続けて実行する
		case PrepareSyntaxError:
			fmt.Println("Syntax error. Could not parse statement.")
			continue
		case PrepareUnrecognizedStatement:
			fmt.Printf(
				"Unrecognized keyword at start of '%s'\n",
				inputBuffer.buffer,
			)
			continue
		}

		executeStatement(&statement)
		fmt.Println("Executed.")
	}
}
