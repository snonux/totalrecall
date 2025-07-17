#!/bin/bash

# Test script for the phonetic information GUI feature

echo "Testing TotalRecall GUI with phonetic information feature..."
echo ""
echo "Features to test:"
echo "1. Enter Bulgarian words like: ябълка (apple), котка (cat), куче (dog)"
echo "2. Phonetic info shows detailed IPA with pronunciation examples for EACH letter"
echo "3. Examples compare to English sounds (e.g., '/a/ like in father')"
echo "4. Phonetic info is saved automatically and persists after restart"
echo "5. Use arrow keys or prev/next buttons to navigate between cards"
echo "6. Info fetches concurrently with audio/image for faster processing"
echo ""
echo "The phonetic text area is located between the image section and audio controls."
echo ""
echo "Press Enter to start the GUI..."
read

# Run the GUI
./totalrecall gui