#!/bin/bash

# Test scene generation with Bulgarian words
echo "Testing scene generation for Bulgarian flashcards..."

# Test words
words=("ябълка" "котка" "куче" "хляб" "вода" "книга")

for word in "${words[@]}"; do
    echo ""
    echo "===================="
    echo "Testing word: $word"
    echo "===================="
    go run ./cmd/totalrecall "$word" --provider openai
    echo ""
    echo "Press Enter to continue to next word..."
    read
done

echo "Scene generation test complete!"