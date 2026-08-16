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

type NodeType byte

const (
	NodeInternal NodeType = iota
	NodeLeaf
)

const (
	nodeTypeSize   = 1
	nodeTypeOffset = 0

	isRootSize   = 1
	isRootOffset = nodeTypeOffset + nodeTypeSize

	parentPointerSize   = 4
	parentPointerOffset = isRootOffset + isRootSize

	commonNodeHeaderSize = nodeTypeSize + isRootSize + parentPointerSize

	internalNodeNumKeysSize   = 4
	internalNodeNumKeysOffset = commonNodeHeaderSize

	internalNodeRightChildSize   = 4
	internalNodeRightChildOffset = internalNodeNumKeysOffset + internalNodeNumKeysSize

	internalNodeHeaderSize = commonNodeHeaderSize + internalNodeNumKeysSize + internalNodeRightChildSize

	internalNodeChildSize = 4
	internalNodeKeySize   = 4
	internalNodeCellSize  = internalNodeChildSize + internalNodeKeySize

	leafNodeNumCellsSize   = 4
	leafNodeNumCellsOffset = commonNodeHeaderSize
	leafNodeHeaderSize     = commonNodeHeaderSize + leafNodeNumCellsSize

	leafNodeKeySize   = 4
	leafNodeKeyOffset = 0

	leafNodeValueSize   = rowSize
	leafNodeValueOffset = leafNodeKeyOffset + leafNodeKeySize

	leafNodeCellSize      = leafNodeKeySize + leafNodeValueSize
	leafNodeSpaceForCells = pageSize - leafNodeHeaderSize
	leafNodeMaxCells      = leafNodeSpaceForCells / leafNodeCellSize

	leafNodeRightSplitCount = (leafNodeMaxCells + 1) / 2
	leafNodeLeftSplitCount  = (leafNodeMaxCells + 1) - leafNodeRightSplitCount
)

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
)

type Row struct {
	id       uint32
	username string
	email    string
}

type ExecuteResult int

const (
	ExecuteSuccess ExecuteResult = iota
	ExecuteDuplicateKey
)

type Pager struct {
	file       *os.File
	fileLength int64
	numPages   uint32
	pages      [tableMaxPages][]byte
}

type Table struct {
	pager       *Pager
	rootPageNum uint32
}

type Cursor struct {
	table      *Table
	pageNum    uint32
	cellNum    uint32
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

	if inputBuffer.buffer == ".btree" {
		fmt.Println("Tree:")
		printTree(table.pager, table.rootPageNum, 0)
		return MetaCommandSuccess
	}

	if inputBuffer.buffer == ".constants" {
		fmt.Println("Constants:")
		printConstants()
		return MetaCommandSuccess
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

func printConstants() {
	fmt.Printf("ROW_SIZE: %d\n", rowSize)
	fmt.Printf("COMMON_NODE_HEADER_SIZE: %d\n", commonNodeHeaderSize)
	fmt.Printf("LEAF_NODE_HEADER_SIZE: %d\n", leafNodeHeaderSize)
	fmt.Printf("LEAF_NODE_CELL_SIZE: %d\n", leafNodeCellSize)
	fmt.Printf("LEAF_NODE_SPACE_FOR_CELLS: %d\n", leafNodeSpaceForCells)
	fmt.Printf("LEAF_NODE_MAX_CELLS: %d\n", leafNodeMaxCells)
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

func leafNodeNumCells(node []byte) uint32 {
	return binary.LittleEndian.Uint32(
		node[leafNodeNumCellsOffset : leafNodeNumCellsOffset+leafNodeNumCellsSize],
	)
}

func setLeafNodeNumCells(node []byte, numCells uint32) {
	binary.LittleEndian.PutUint32(
		node[leafNodeNumCellsOffset:leafNodeNumCellsOffset+leafNodeNumCellsSize],
		numCells,
	)
}

func getNodeType(node []byte) NodeType {
	return NodeType(node[nodeTypeOffset])
}

func setNodeType(node []byte, nodeType NodeType) {
	node[nodeTypeOffset] = byte(nodeType)
}

func isNodeRoot(node []byte) bool {
	return node[isRootOffset] != 0
}

func setNodeRoot(node []byte, isRoot bool) {
	if isRoot {
		node[isRootOffset] = 1
		return
	}

	node[isRootOffset] = 0
}

func internalNodeNumKeys(node []byte) uint32 {
	return binary.LittleEndian.Uint32(
		node[internalNodeNumKeysOffset : internalNodeNumKeysOffset+internalNodeNumKeysSize],
	)
}

func setInternalNodeNumKeys(node []byte, numKeys uint32) {
	binary.LittleEndian.PutUint32(
		node[internalNodeNumKeysOffset:internalNodeNumKeysOffset+internalNodeNumKeysSize],
		numKeys,
	)
}

func internalNodeRightChild(node []byte) uint32 {
	return binary.LittleEndian.Uint32(
		node[internalNodeRightChildOffset : internalNodeRightChildOffset+internalNodeRightChildSize],
	)
}

func leafNodeCell(node []byte, cellNum uint32) []byte {
	cellOffset := leafNodeHeaderSize + cellNum*leafNodeCellSize
	return node[cellOffset : cellOffset+leafNodeCellSize]
}

func setInternalNodeRightChild(node []byte, pageNum uint32) {
	binary.LittleEndian.PutUint32(
		node[internalNodeRightChildOffset:internalNodeRightChildOffset+internalNodeRightChildSize],
		pageNum,
	)
}

func internalNodeCell(node []byte, cellNum uint32) []byte {
	cellOffset := internalNodeHeaderSize + cellNum*internalNodeCellSize
	return node[cellOffset : cellOffset+internalNodeCellSize]
}

func internalNodeChild(node []byte, childNum uint32) uint32 {
	numKeys := internalNodeNumKeys(node)

	if childNum > numKeys {
		fmt.Fprintf(
			os.Stderr,
			"Tried to access child number %d > number of keys %d\n",
			childNum,
			numKeys,
		)
		os.Exit(1)
	}

	if childNum == numKeys {
		return internalNodeRightChild(node)
	}

	cell := internalNodeCell(node, childNum)
	return binary.LittleEndian.Uint32(cell[:internalNodeChildSize])
}

func setInternalNodeChild(node []byte, childNum uint32, pageNum uint32) {
	numKeys := internalNodeNumKeys(node)

	if childNum > numKeys {
		fmt.Fprintf(
			os.Stderr,
			"Tried to access child number %d > number of keys %d\n",
			childNum,
			numKeys,
		)
		os.Exit(1)
	}

	if childNum == numKeys {
		setInternalNodeRightChild(node, pageNum)
		return
	}

	cell := internalNodeCell(node, childNum)
	binary.LittleEndian.PutUint32(cell[:internalNodeChildSize], pageNum)
}

func internalNodeKey(node []byte, keyNum uint32) uint32 {
	cell := internalNodeCell(node, keyNum)
	return binary.LittleEndian.Uint32(
		cell[internalNodeChildSize : internalNodeChildSize+internalNodeKeySize],
	)
}

func setInternalNodeKey(node []byte, keyNum uint32, key uint32) {
	cell := internalNodeCell(node, keyNum)
	binary.LittleEndian.PutUint32(
		cell[internalNodeChildSize:internalNodeChildSize+internalNodeKeySize],
		key,
	)
}

func getNodeMaxKey(node []byte) uint32 {
	switch getNodeType(node) {
	case NodeInternal:
		return internalNodeKey(node, internalNodeNumKeys(node)-1)
	case NodeLeaf:
		return leafNodeKey(node, leafNodeNumCells(node)-1)
	default:
		panic("unknown node type")
	}
}

func leafNodeKey(node []byte, cellNum uint32) uint32 {
	cell := leafNodeCell(node, cellNum)
	return binary.LittleEndian.Uint32(
		cell[leafNodeKeyOffset : leafNodeKeyOffset+leafNodeKeySize],
	)
}

func setLeafNodeKey(node []byte, cellNum uint32, key uint32) {
	cell := leafNodeCell(node, cellNum)
	binary.LittleEndian.PutUint32(
		cell[leafNodeKeyOffset:leafNodeKeyOffset+leafNodeKeySize],
		key,
	)
}

func leafNodeValue(node []byte, cellNum uint32) []byte {
	cell := leafNodeCell(node, cellNum)
	return cell[leafNodeValueOffset : leafNodeValueOffset+leafNodeValueSize]
}

func initializeLeafNode(node []byte) {
	setNodeType(node, NodeLeaf)
	setNodeRoot(node, false)
	setLeafNodeNumCells(node, 0)
}

func initializeInternalNode(node []byte) {
	setNodeType(node, NodeInternal)
	setNodeRoot(node, false)
	setInternalNodeNumKeys(node, 0)
}

func createNewRoot(table *Table, rightChildPageNum uint32) {
	root := getPage(table.pager, table.rootPageNum)
	rightChild := getPage(table.pager, rightChildPageNum)

	leftChildPageNum := getUnusedPageNum(table.pager)
	leftChild := getPage(table.pager, leftChildPageNum)

	// 元のルートの内容を左の子へ退避させる
	copy(leftChild, root)
	setNodeRoot(leftChild, false)

	// ページ0を新しい内部ルートとして再利用する
	initializeInternalNode(root)
	setNodeRoot(root, true)
	setInternalNodeNumKeys(root, 1)
	setInternalNodeChild(root, 0, leftChildPageNum)
	setInternalNodeKey(root, 0, getNodeMaxKey(leftChild))
	setInternalNodeRightChild(root, rightChildPageNum)

	// rightChild は将来の親ポインタ実装で使うため、ここでは参照だけ行う
	_ = rightChild
}

func leafNodeSplitAndInsert(cursor *Cursor, key uint32, value *Row) {
	oldNode := getPage(cursor.table.pager, cursor.pageNum)
	newPageNum := getUnusedPageNum(cursor.table.pager)
	newNode := getPage(cursor.table.pager, newPageNum)
	initializeLeafNode(newNode)

	// 既存13セルと新規1セルを、右端から正しい位置へ再配置する
	for i := int32(leafNodeMaxCells); i >= 0; i-- {
		var destinationNode []byte

		if uint32(i) >= leafNodeLeftSplitCount {
			destinationNode = newNode
		} else {
			destinationNode = oldNode
		}

		indexWithinNode := uint32(i) % leafNodeLeftSplitCount
		destination := leafNodeCell(destinationNode, indexWithinNode)

		switch {
		case uint32(i) == cursor.cellNum:
			setLeafNodeKey(destinationNode, indexWithinNode, key)
			serializeRow(value, leafNodeValue(destinationNode, indexWithinNode))
		case uint32(i) > cursor.cellNum:
			copy(destination, leafNodeCell(oldNode, uint32(i)-1))
		default:
			copy(destination, leafNodeCell(oldNode, uint32(i)))
		}
	}

	setLeafNodeNumCells(oldNode, leafNodeLeftSplitCount)
	setLeafNodeNumCells(newNode, leafNodeRightSplitCount)

	if isNodeRoot(oldNode) {
		createNewRoot(cursor.table, newPageNum)
		return
	}

	fmt.Fprintln(os.Stderr, "Need to implement updating parent after split.")
	os.Exit(1)
}

func leafNodeInsert(cursor *Cursor, key uint32, value *Row) {
	node := getPage(cursor.table.pager, cursor.pageNum)
	numCells := leafNodeNumCells(node)

	if numCells >= leafNodeMaxCells {
		leafNodeSplitAndInsert(cursor, key, value)
		return
	}

	if cursor.cellNum < numCells {
		for cellNum := numCells; cellNum > cursor.cellNum; cellNum-- {
			copy(
				leafNodeCell(node, cellNum),
				leafNodeCell(node, cellNum-1),
			)
		}
	}

	setLeafNodeNumCells(node, numCells+1)
	setLeafNodeKey(node, cursor.cellNum, key)
	serializeRow(value, leafNodeValue(node, cursor.cellNum))
}

func indent(level uint32) {
	for i := uint32(0); i < level; i++ {
		fmt.Print("  ")
	}
}

func printTree(pager *Pager, pageNum uint32, indentationLevel uint32) {
	node := getPage(pager, pageNum)

	switch getNodeType(node) {
	case NodeLeaf:
		numKeys := leafNodeNumCells(node)
		indent(indentationLevel)
		fmt.Printf("- leaf (size %d)\n", numKeys)

		for i := uint32(0); i < numKeys; i++ {
			indent(indentationLevel + 1)
			fmt.Printf("- %d\n", leafNodeKey(node, i))
		}
	case NodeInternal:
		numKeys := internalNodeNumKeys(node)
		indent(indentationLevel)
		fmt.Printf("- internal (size %d)\n", numKeys)

		for i := uint32(0); i < numKeys; i++ {
			printTree(pager, internalNodeChild(node, i), indentationLevel+1)
			indent(indentationLevel + 1)
			fmt.Printf("- key %d\n", internalNodeKey(node, i))
		}

		printTree(
			pager,
			internalNodeRightChild(node),
			indentationLevel+1,
		)
	}
}

func tableStart(table *Table) *Cursor {
	rootNode := getPage(table.pager, table.rootPageNum)

	return &Cursor{
		table:      table,
		pageNum:    table.rootPageNum,
		cellNum:    0,
		endOfTable: leafNodeNumCells(rootNode) == 0,
	}
}

// tableFind returns the key's position, or the position where it belongs.
func tableFind(table *Table, key uint32) *Cursor {
	rootPageNum := table.rootPageNum
	rootNode := getPage(table.pager, rootPageNum)

	if getNodeType(rootNode) == NodeLeaf {
		return leafNodeFind(table, rootPageNum, key)
	}

	return internalNodeFind(table, rootPageNum, key)
}

func leafNodeFind(table *Table, pageNum uint32, key uint32) *Cursor {
	node := getPage(table.pager, pageNum)
	numCells := leafNodeNumCells(node)

	cursor := &Cursor{
		table:   table,
		pageNum: pageNum,
	}

	minIndex := uint32(0)
	onePastMaxIndex := numCells

	for onePastMaxIndex != minIndex {
		index := (minIndex + onePastMaxIndex) / 2
		keyAtIndex := leafNodeKey(node, index)

		if key == keyAtIndex {
			cursor.cellNum = index
			return cursor
		}

		if key < keyAtIndex {
			onePastMaxIndex = index
		} else {
			minIndex = index + 1
		}
	}

	cursor.cellNum = minIndex
	return cursor
}

func internalNodeFind(table *Table, pageNum uint32, key uint32) *Cursor {
	node := getPage(table.pager, pageNum)
	numKeys := internalNodeNumKeys(node)

	// キー数より子ノード数は1つ多い
	minIndex := uint32(0)
	maxIndex := numKeys

	// key 以上の最初の内部ノードキーを探す
	for minIndex != maxIndex {
		index := (minIndex + maxIndex) / 2
		keyToRight := internalNodeKey(node, index)

		if keyToRight >= key {
			maxIndex = index
		} else {
			minIndex = index + 1
		}
	}

	childPageNum := internalNodeChild(node, minIndex)
	child := getPage(table.pager, childPageNum)

	switch getNodeType(child) {
	case NodeLeaf:
		return leafNodeFind(table, childPageNum, key)
	case NodeInternal:
		return internalNodeFind(table, childPageNum, key)
	default:
		panic("unknown node type")
	}
}

func cursorValue(cursor *Cursor) []byte {
	node := getPage(cursor.table.pager, cursor.pageNum)
	return leafNodeValue(node, cursor.cellNum)
}

func cursorAdvance(cursor *Cursor) {
	node := getPage(cursor.table.pager, cursor.pageNum)
	cursor.cellNum++

	if cursor.cellNum >= leafNodeNumCells(node) {
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

	if fileInfo.Size()%pageSize != 0 {
		fmt.Fprintln(os.Stderr, "Db file is not a whole number of pages.")
		os.Exit(1)
	}

	return &Pager{
		file:       file,
		fileLength: fileInfo.Size(),
		numPages:   uint32(fileInfo.Size() / pageSize),
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

		if pageNum >= pager.numPages {
			pager.numPages = pageNum + 1
		}
	}

	return pager.pages[pageNum]
}

func getUnusedPageNum(pager *Pager) uint32 {
	return pager.numPages
}

func dbOpen(filename string) *Table {
	pager := pagerOpen(filename)
	table := &Table{
		pager:       pager,
		rootPageNum: 0,
	}

	if pager.numPages == 0 {
		rootNode := getPage(pager, table.rootPageNum)
		initializeLeafNode(rootNode)
		setNodeRoot(rootNode, true)
	}

	return table
}

func pagerFlush(pager *Pager, pageNum uint32) {
	page := pager.pages[pageNum]
	if page == nil {
		fmt.Fprintln(os.Stderr, "Tried to flush an unloaded page.")
		os.Exit(1)
	}

	_, err := pager.file.WriteAt(page, int64(pageNum)*pageSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error writing file:", err)
		os.Exit(1)
	}
}

func dbClose(table *Table) {
	pager := table.pager

	for pageNum := uint32(0); pageNum < pager.numPages; pageNum++ {
		if pager.pages[pageNum] != nil {
			pagerFlush(pager, pageNum)
		}
	}

	if err := pager.file.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "Error closing database file:", err)
		os.Exit(1)
	}
}

func executeInsert(statement *Statement, table *Table) ExecuteResult {
	node := getPage(table.pager, table.rootPageNum)
	numCells := leafNodeNumCells(node)

	rowToInsert := &statement.rowToInsert
	keyToInsert := rowToInsert.id
	cursor := tableFind(table, keyToInsert)

	if cursor.cellNum < numCells {
		keyAtIndex := leafNodeKey(node, cursor.cellNum)
		if keyAtIndex == keyToInsert {
			return ExecuteDuplicateKey
		}
	}

	leafNodeInsert(cursor, rowToInsert.id, rowToInsert)

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
		case ExecuteDuplicateKey:
			fmt.Println("Error: Duplicate key.")
		}
	}
}
