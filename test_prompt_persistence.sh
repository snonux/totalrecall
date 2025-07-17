#!/bin/bash
# Test script to verify that image prompts are preserved during rapid navigation

echo "Starting totalrecall GUI to test prompt persistence..."
echo ""
echo "Test procedure:"
echo "1. Enter multiple words quickly with custom prompts:"
echo "   - Word 1: 'ябълка' with prompt 'red apple on wooden table'"
echo "   - Word 2: 'котка' with prompt 'orange cat sleeping on sofa'"
echo "   - Word 3: 'куче' with prompt 'golden retriever playing in park'"
echo ""
echo "2. While images are still generating, navigate rapidly using arrow keys"
echo "3. Verify that:"
echo "   - Each word retains its correct custom prompt"
echo "   - Prompts don't get mixed up between words"
echo "   - Prompts are saved to disk (*_prompt.txt files)"
echo ""
echo "4. After all processing completes, navigate through words again"
echo "5. Verify prompts are correctly loaded from disk"
echo ""
echo "Check the anki_cards folder for *_prompt.txt files"
echo ""
echo "Press Ctrl+C to exit when testing is complete."
echo ""

./totalrecall gui