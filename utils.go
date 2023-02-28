package main

import (
	"bufio"
	"fmt"
	"os"
)

func readLines(filePath string) ([]string, error) {
	// import file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading from file: %w", err)
	}

	defer file.Close()

	lines := []string{}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	return lines, nil
}
