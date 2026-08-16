package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
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
	PrepareNegativeID
	PrepareStringTooLong
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

type Pager struct {
	file       *os.File
	fileLength int64
	pages      [tableMaxPages][]byte
}

type Table struct {
	numRows uint32
	pager   *Pager
}

type Cursor struct {
	table      *Table
	rowNum     uint32
	endOfTable bool
}

type Statement struct {
	statementType StatementType
	rowToInsert   Row
}

func doMetaCommand(
	inputBuffer *InputBuffer,
	table *Table) MetaCommandResult {
	if inputBuffer.buffer == ".exit" {
		dbClose(table)
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

		id, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return PrepareSyntaxError
		}

		if id < 0 {
			return PrepareNegativeID
		}

		if len(fields[2]) > columnUsernameSize {
			return PrepareStringTooLong
		}

		if len(fields[3]) > columnEmailSize {
			return PrepareStringTooLong
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
		destination[idOffset:idOffset+idSize],
		source.id,
	)
	copy(
		destination[usernameOffset:usernameOffset+usernameSize],
		source.username,
	)
	copy(
		destination[emailOffset:emailOffset+emailSize],
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
	page := getPage(table.pager, pageNum)

	rowOffset := rowNum % rowsPerPage
	byteOffset := rowOffset * rowSize

	return page[byteOffset : byteOffset+rowSize]
}

func tableStart(table *Table) *Cursor {
	return &Cursor{
		table:      table,
		rowNum:     0,
		endOfTable: table.numRows == 0,
	}
}

func tableEnd(table *Table) *Cursor {
	return &Cursor{
		table:      table,
		rowNum:     table.numRows,
		endOfTable: true,
	}
}

func cursorValue(cursor *Cursor) []byte {
	return rowSlot(cursor.table, cursor.rowNum)
}

func cursorAdvance(cursor *Cursor) {
	cursor.rowNum++
	if cursor.rowNum >= cursor.table.numRows {
		cursor.endOfTable = true
	}
}

func pagerOpen(filename string) *Pager {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to open file:", err)
		os.Exit(1)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to read file information:", err)
		os.Exit(1)
	}

	return &Pager{
		file:       file,
		fileLength: fileInfo.Size(),
	}
}

func getPage(pager *Pager, pageNum uint32) []byte {
	if pageNum >= tableMaxPages {
		fmt.Fprintf(
			os.Stderr,
			"Tried to fetch page number out of bounds: %d\n",
			pageNum,
		)
		os.Exit(1)
	}

	if pager.pages[pageNum] == nil {
		page := make([]byte, pageSize)
		pageOffset := int64(pageNum) * pageSize

		if pageOffset < pager.fileLength {
			_, err := pager.file.ReadAt(page, pageOffset)
			if err != nil && err != io.EOF {
				fmt.Fprintln(os.Stderr, "Error reading file:", err)
				os.Exit(1)
			}
		}

		pager.pages[pageNum] = page
	}

	return pager.pages[pageNum]
}

func dbOpen(filename string) *Table {
	pager := pagerOpen(filename)

	return &Table{
		pager:   pager,
		numRows: uint32(pager.fileLength / rowSize),
	}
}

func pagerFlush(pager *Pager, pageNum uint32, size int) {
	page := pager.pages[pageNum]
	if page == nil {
		fmt.Fprintln(os.Stderr, "Tried to flush an unloaded page.")
		os.Exit(1)
	}

	_, err := pager.file.WriteAt(page[:size], int64(pageNum)*pageSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error writing file:", err)
		os.Exit(1)
	}
}

func dbClose(table *Table) {
	pager := table.pager
	numFullPages := table.numRows / rowsPerPage

	for pageNum := uint32(0); pageNum < numFullPages; pageNum++ {
		if pager.pages[pageNum] != nil {
			pagerFlush(pager, pageNum, pageSize)
		}
	}

	numAdditionalRows := table.numRows % rowsPerPage
	if numAdditionalRows > 0 {
		pageNum := numFullPages
		if pager.pages[pageNum] != nil {
			pagerFlush(
				pager,
				pageNum,
				int(numAdditionalRows*rowSize),
			)
		}
	}

	if err := pager.file.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "Error closing database file:", err)
		os.Exit(1)
	}
}

func executeInsert(statement *Statement, table *Table) ExecuteResult {
	if table.numRows >= tableMaxRows {
		return ExecuteTableFull
	}

	cursor := tableEnd(table)
	rowToInsert := &statement.rowToInsert
	serializeRow(rowToInsert, cursorValue(cursor))
	table.numRows++

	return ExecuteSuccess
}

func executeSelect(_ *Statement, table *Table) ExecuteResult {
	cursor := tableStart(table)

	for !cursor.endOfTable {
		var row Row
		deserializeRow(cursorValue(cursor), &row)
		printRow(&row)
		cursorAdvance(cursor)
	}

	return ExecuteSuccess
}

func executeStatement(statement *Statement, table *Table) ExecuteResult {
	switch statement.statementType {
	case StatementInsert:
		return executeInsert(statement, table)
	case StatementSelect:
		return executeSelect(statement, table)
	}

	return ExecuteSuccess
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
	if len(os.Args) < 2 {
		fmt.Println("Must supply a database filename.")
		return
	}

	table := dbOpen(os.Args[1])
	inputBuffer := &InputBuffer{}
	reader := bufio.NewReader(os.Stdin)

	for {
		printPrompt()

		if err := readInput(inputBuffer, reader); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input")
			return
		}

		if strings.HasPrefix(inputBuffer.buffer, ".") {
			switch doMetaCommand(inputBuffer, table) {
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
		case PrepareNegativeID:
			fmt.Println("ID must be positive.")
			continue
		case PrepareStringTooLong:
			fmt.Println("String is too long.")
			continue
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

		switch executeStatement(&statement, table) {
		case ExecuteSuccess:
			fmt.Println("Executed.")
		case ExecuteTableFull:
			fmt.Println("Error: Table full.")
		}
	}
}
