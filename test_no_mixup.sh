#!/bin/bash
# Test script to verify that rapid word entry doesn't cause mix-ups

echo "Starting totalrecall GUI to test rapid word entry..."
echo ""
echo "Test procedure:"
echo "1. Enter a word (e.g., 'ябълка') and click Generate"
echo "2. While it's still processing, immediately enter a new word (e.g., 'котка')"
echo "3. Click Generate for the second word"
echo "4. Verify that:"
echo "   - The first word's image/audio appears correctly when it completes"
echo "   - The second word's image/audio appears correctly when it completes"
echo "   - No mix-ups occur between the two words"
echo "5. Check the anki_cards folder to ensure files are correctly named"
echo ""
echo "Additional tests:"
echo "- Try entering 'rocket launcher' in English, then quickly enter another word"
echo "- Verify the translation doesn't get mixed up"
echo ""
echo "Press Ctrl+C to exit when testing is complete."
echo ""

./totalrecall gui