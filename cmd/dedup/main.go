package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

func main() {
	// Open the input file for reading
	inputFile, err := os.Open("test.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer inputFile.Close()

	// Create the output file for writing
	outputFile, err := os.Create("output.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	// Create a map to store unique lines
	seen := make(map[string]bool)

	// Create a new scanner for the input file
	scanner := bufio.NewScanner(inputFile)

	// Create a new writer for the output file
	writer := bufio.NewWriter(outputFile)

	// Loop through each line of the input file
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line has been seen before
		if _, ok := seen[line]; !ok {
			// If not, mark it as seen
			seen[line] = true
			// And write it to the output file
			fmt.Fprintln(writer, line)
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Flush the writer to ensure all buffered data is written to the file
	writer.Flush()
}
