package utils

import (
	"bufio"
	"os"
)

func ReadFromLine(path string, startLine int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1

	var result []string
	for scanner.Scan() {
		if lineNum >= startLine {
			result = append(result, scanner.Text())
		}
		lineNum++
	}

	return result, scanner.Err()
}
