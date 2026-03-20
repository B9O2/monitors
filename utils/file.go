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
	const maxCapacity = 1 * 1024 * 1024 // 1MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)
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
