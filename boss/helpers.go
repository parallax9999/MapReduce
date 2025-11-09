package main

import (
	"fmt"
	"io"
	"os"
)

// getLineAlignedByteRanges returns byte ranges aligned to line boundaries for CSV files
func (boss *BossState) getLineAlignedByteRanges(filePath string, fileSize int64, numRanges int) ([]struct{ start, end int64 }, error) {
	// Use VOLUME_PATH environment variable
	volumePath := os.Getenv("VOLUME_PATH")
	if volumePath == "" {
		volumePath = "./volume" // fallback to relative path
	}
	fullPath := volumePath + filePath
	
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", fullPath, err)
	}
	defer file.Close()

	if numRanges <= 0 {
		return nil, fmt.Errorf("numRanges must be positive")
	}
	if numRanges == 1 {
		return []struct{ start, end int64 }{{0, fileSize}}, nil
	}

	targetSize := fileSize / int64(numRanges)
	ranges := make([]struct{ start, end int64 }, 0, numRanges)
	buffer := make([]byte, 1024*1024) // 1MB buffer
	
	currentStart := int64(0)
	
	for i := 1; i < numRanges; i++ {
		targetEnd := int64(i) * targetSize
		
		// Find the next newline after targetEnd
		actualEnd, err := boss.findNextNewline(file, targetEnd, fileSize, buffer)
		if err != nil {
			return nil, err
		}
		
		ranges = append(ranges, struct{ start, end int64 }{currentStart, actualEnd})
		currentStart = actualEnd
	}
	
	// Last range goes to end of file
	ranges = append(ranges, struct{ start, end int64 }{currentStart, fileSize})
	
	return ranges, nil
}

// findNextNewline finds the next newline character at or after the target position
func (boss *BossState) findNextNewline(file *os.File, targetPos, fileSize int64, buffer []byte) (int64, error) {
	if targetPos >= fileSize {
		return fileSize, nil
	}
	
	bufferSize := int64(len(buffer))
	pos := targetPos
	
	for pos < fileSize {
		// Read buffer starting at pos
		readSize := bufferSize
		if pos + readSize > fileSize {
			readSize = fileSize - pos
		}
		
		n, err := file.ReadAt(buffer[:readSize], pos)
		if err != nil && err != io.EOF {
			return 0, fmt.Errorf("failed to read at position %d: %v", pos, err)
		}
		
		// Look for newline in buffer
		for i := 0; i < n; i++ {
			if buffer[i] == '\n' {
				return pos + int64(i) + 1, nil // Position after the newline
			}
		}
		
		// Move to next buffer position
		pos += int64(n)
		
		// If we didn't read a full buffer, we're at EOF
		if n < int(readSize) {
			break
		}
	}
	
	// No newline found, return end of file
	return fileSize, nil
}