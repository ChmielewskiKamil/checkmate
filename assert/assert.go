package assert

import (
	"fmt"
	"os"
)

// NotNil checks that the given value is not nil.
// It panics with an Assertion Failed message if the value is nil.
func NotNil(i interface{}, message ...string) {
	if i == nil {
		if len(message) > 0 {
			panic(prefixMessage(message[0]))
		}
		panic(prefixMessage("Expected a non-nil value, but got nil."))
	}
}

// Nil checks that the given value is nil.
// It panics with an Assertion Failed message if the value is not nil.
func Nil(i interface{}, message ...string) {
	if i != nil {
		if len(message) > 0 {
			panic(prefixMessage(message[0]))
		}
		panic(prefixMessage("Expected a nil value, but got non-nil."))
	}
}

// PathExists checks if a file or directory exists at the given path.
// It panics with an Assertion Failed message if the path does not exist.
func PathExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic(prefixMessage(fmt.Sprintf("Path does not exist: %s", path)))
	} else if err != nil {
		panic(prefixMessage(fmt.Sprintf("Error accessing path: %v", err)))
	}
}

// PathNotExists checks that a file or directory does not exist at the given path.
// It panics with an Assertion Failed message if the path does exist.
func PathNotExists(path string) {
	if _, err := os.Stat(path); err == nil {
		panic(prefixMessage(fmt.Sprintf("Path exists BUT shouldn't: %s", path)))
	}
}

// NotEmpty checks if a directory or file at the given path is not empty.
// It panics with an Assertion Failed message if the path is empty or does not exist.
// WARNING NotEmpty won't panic if directory contains subdirectories which are empty.
func NotEmpty(path string) {
	// Pre-conditions
	PathExists(path)

	info, _ := os.Stat(path)

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			panic(prefixMessage(fmt.Sprintf("Error reading directory: %v", err)))
		}
		if len(entries) == 0 {
			panic(prefixMessage(fmt.Sprintf("Directory is empty: %s", path)))
		}
	} else {
		if info.Size() == 0 {
			panic(prefixMessage(fmt.Sprintf("File is empty: %s", path)))
		}
	}
}

// Empty checks if a directory or file at the given path is empty.
// It panics with an Assertion Failed message if the path is not empty or does not exist.
func Empty(path string) {
	// Pre-condition
	PathExists(path)

	info, _ := os.Stat(path)

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			panic(prefixMessage(fmt.Sprintf("Error reading directory: %v", err)))
		}
		if len(entries) > 0 {
			panic(prefixMessage(fmt.Sprintf("Directory is not empty: %s", path)))
		}
	} else {
		if info.Size() > 0 {
			panic(prefixMessage(fmt.Sprintf("File is not empty: %s", path)))
		}
	}
}

func True(condition bool, message string) {
	if !condition {
		panic(prefixMessage(message))
	}
}

func prefixMessage(customMessage string) string {
	return fmt.Sprintf("Assertion failed: %s", customMessage)
}
