#!/bin/bash
# Test script to verify single image functionality

echo "Building totalrecall..."
go build -o totalrecall ./cmd/totalrecall || exit 1

echo "Testing CLI mode with a single word..."
./totalrecall "котка" || exit 1

echo "Checking generated files..."
ls -la output/котка* 

echo "Test completed successfully!"