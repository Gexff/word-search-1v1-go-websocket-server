package game

import (
    "bufio"
    "log"
    "os"
)

func ReadWords(filepath string) []string {
    // Open the file
    file, err := os.Open(filepath) // Make sure the file path is correct
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Create a slice to hold the words
    var words []string

    // Read the file line by line
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        if line != "" { // ignore empty lines
            words = append(words, line)
        }
    }

	return words
}