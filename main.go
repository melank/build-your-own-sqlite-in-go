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

        if inputBuffer.buffer == ".exit" {
            return
        }

        fmt.Printf("Unrecognized command '%s' .\n", inputBuffer.buffer)
    }
}