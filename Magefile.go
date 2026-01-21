//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

var Default = Build

// Build compiles the totalrecall binary
func Build() error {
	return sh.RunV("go", "build", "-o", "totalrecall", "./cmd/totalrecall")
}

// Run executes the application without building
func Run() error {
	return sh.RunV("go", "run", "./cmd/totalrecall")
}

// Test runs all tests
func Test() error {
	return sh.RunV("go", "test", "./...")
}

// TestVerbose runs all tests with verbose output
func TestVerbose() error {
	return sh.RunV("go", "test", "-v", "./...")
}

// TestCoverage runs all tests with coverage report
func TestCoverage() error {
	return sh.RunV("go", "test", "-cover", "./...")
}

// TestCoverageHtml runs all tests and generates HTML coverage report
func TestCoverageHtml() error {
	if err := sh.RunV("go", "test", "-coverprofile=coverage.out", "./..."); err != nil {
		return err
	}
	if err := sh.RunV("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		return err
	}
	fmt.Println("Coverage report generated at coverage.html")
	return nil
}

// TestRace runs all tests with race detector
func TestRace() error {
	return sh.RunV("go", "test", "-race", "./...")
}

// TestShort runs only short tests (skip integration tests)
func TestShort() error {
	return sh.RunV("go", "test", "-short", "./...")
}

// TestAll runs comprehensive test suite with coverage and race detection
func TestAll() error {
	fmt.Println("Running comprehensive test suite...")
	return sh.RunV("go", "test", "-v", "-race", "-cover", "./...")
}

// Install installs the binary to Go bin directory
func Install() error {
	return sh.RunV("go", "install", "./cmd/totalrecall")
}

// Clean removes build artifacts and test coverage files
func Clean() error {
	files := []string{"totalrecall", "coverage.out", "coverage.html"}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return sh.RunV("go", "clean", "-testcache")
}
