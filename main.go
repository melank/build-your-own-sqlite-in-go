package main

import (
    "bufio"
    "fmt"
    "os"
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
    PrepareUnrecognizedStatement
)

type StatementType int

const (
    StatementInsert StatementType = iota
    StatementSelect
)

type Statement struct {
    statementType StatementType
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
        return PrepareSuccess
    }

    if inputBuffer.buffer == "select" {
        statement.statementType = StatementSelect
        return PrepareSuccess
    }

    return PrepareUnrecognizedStatement
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