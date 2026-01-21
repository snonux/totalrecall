package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TestState mimics the Application state for a bg-bg card
type TestState struct {
	currentWord        string
	currentCardType    string
	currentTranslation string
	cardDir            string
}

// SimulateKey_a simulates pressing 'a' (regenerate front audio)
func (ts *TestState) SimulateKey_a() {
	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Println("SIMULATING: Key pressed 'a' (regenerate front audio)")
	fmt.Println(repeatStr("=", 80))

	fmt.Printf("Conditions check:\n")
	fmt.Printf("  - currentWord: %s\n", ts.currentWord)
	fmt.Printf("  - currentCardType: %s\n", ts.currentCardType)

	if ts.currentCardType != "bg-bg" {
		fmt.Printf("  ✗ NOT a bg-bg card, would return\n")
		return
	}

	fmt.Printf("  ✓ Is bg-bg card, proceeding...\n")

	// Simulate generateAudioFront
	frontFile := filepath.Join(ts.cardDir, "audio_front.mp3")
	fmt.Printf("\nGenerating front audio for '%s'\n", ts.currentWord)
	fmt.Printf("  Output file: %s\n", frontFile)

	// Create file
	if err := createAudioFile(frontFile, "FRONT_"+ts.currentWord); err != nil {
		fmt.Printf("  ✗ Error: %v\n", err)
		return
	}

	fmt.Printf("  ✓ Successfully wrote front audio\n")
	fmt.Printf("  📁 File: audio_front.mp3\n")
}

// SimulateKey_A simulates pressing 'A' (regenerate back audio)
func (ts *TestState) SimulateKey_A() {
	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Println("SIMULATING: Key pressed 'A' (regenerate back audio)")
	fmt.Println(repeatStr("=", 80))

	fmt.Printf("Conditions check:\n")
	fmt.Printf("  - currentWord: %s\n", ts.currentWord)
	fmt.Printf("  - currentCardType: %s\n", ts.currentCardType)
	fmt.Printf("  - currentTranslation: %s\n", ts.currentTranslation)

	if ts.currentCardType != "bg-bg" {
		fmt.Printf("  ✗ NOT a bg-bg card, would return\n")
		return
	}

	fmt.Printf("  ✓ Is bg-bg card, proceeding...\n")

	translation := ts.currentTranslation
	if translation == "" {
		fmt.Printf("  ⚠ Translation empty, should fallback to UI field\n")
		return
	}

	fmt.Printf("\nGenerating back audio for '%s'\n", translation)

	// Simulate generateAudioBack
	backFile := filepath.Join(ts.cardDir, "audio_back.mp3")
	fmt.Printf("  Output file: %s\n", backFile)

	// Create file
	if err := createAudioFile(backFile, "BACK_"+translation); err != nil {
		fmt.Printf("  ✗ Error: %v\n", err)
		return
	}

	fmt.Printf("  ✓ Successfully wrote back audio\n")
	fmt.Printf("  📁 File: audio_back.mp3\n")
}

// createAudioFile creates a fake audio file with content marker
func createAudioFile(path, marker string) error {
	content := fmt.Sprintf("FAKE_AUDIO[%s]_GENERATED_AT_%d", marker, time.Now().Unix())
	return os.WriteFile(path, []byte(content), 0644)
}

// VerifyResult checks which files were actually created/modified
func (ts *TestState) VerifyResult() {
	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Println("VERIFICATION: Which audio files were regenerated?")
	fmt.Println(repeatStr("=", 80))

	frontFile := filepath.Join(ts.cardDir, "audio_front.mp3")
	backFile := filepath.Join(ts.cardDir, "audio_back.mp3")

	// Check which files exist and their content
	frontContent, _ := os.ReadFile(frontFile)
	backContent, _ := os.ReadFile(backFile)

	fmt.Printf("\nFront audio (audio_front.mp3):\n")
	if len(frontContent) > 0 {
		fmt.Printf("  ✓ Exists: %s\n", string(frontContent))
		if containsString(string(frontContent), "FRONT_"+ts.currentWord) {
			fmt.Printf("  ✓ CORRECT: Contains front word '%s'\n", ts.currentWord)
		} else if containsString(string(frontContent), "BACK_") {
			fmt.Printf("  ✗ WRONG: Contains back definition (should be front!)\n")
		}
	} else {
		fmt.Printf("  ✗ File does not exist or is empty\n")
	}

	fmt.Printf("\nBack audio (audio_back.mp3):\n")
	if len(backContent) > 0 {
		fmt.Printf("  ✓ Exists: %s\n", string(backContent))
		if containsString(string(backContent), "BACK_"+ts.currentTranslation) {
			fmt.Printf("  ✓ CORRECT: Contains back definition '%s'\n", ts.currentTranslation)
		} else if containsString(string(backContent), "FRONT_") {
			fmt.Printf("  ✗ WRONG: Contains front word (should be back!)\n")
		}
	} else {
		fmt.Printf("  ✗ File does not exist or is empty\n")
	}
}

// TestSequence runs a complete test sequence
func (ts *TestState) TestSequence() {
	fmt.Println("\n" + "╔" + repeatStr("=", 78) + "╗")
	fmt.Println("║" + centerText("TEST: Audio Regeneration for bg-bg Cards", 78) + "║")
	fmt.Println("╚" + repeatStr("=", 78) + "╝")

	fmt.Printf("\nInitial State:\n")
	fmt.Printf("  Word: %s\n", ts.currentWord)
	fmt.Printf("  Card Type: %s\n", ts.currentCardType)
	fmt.Printf("  Translation: %s\n", ts.currentTranslation)
	fmt.Printf("  Card Dir: %s\n", ts.cardDir)

	// Simulate pressing 'a' then 'A'
	ts.SimulateKey_a()
	fmt.Printf("\n[Simulating 'a' completing...]\n")
	time.Sleep(500 * time.Millisecond)

	ts.SimulateKey_A()
	fmt.Printf("\n[Simulating 'A' completing...]\n")
	time.Sleep(500 * time.Millisecond)

	// Verify results
	ts.VerifyResult()

	// Summary
	printSummary(ts)
}

func printSummary(ts *TestState) {
	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Println("SUMMARY")
	fmt.Println(repeatStr("=", 80))

	frontFile := filepath.Join(ts.cardDir, "audio_front.mp3")
	backFile := filepath.Join(ts.cardDir, "audio_back.mp3")

	frontContent, _ := os.ReadFile(frontFile)
	backContent, _ := os.ReadFile(backFile)

	frontCorrect := containsString(string(frontContent), "FRONT_"+ts.currentWord)
	backCorrect := containsString(string(backContent), "BACK_"+ts.currentTranslation)

	fmt.Printf("\nTest Result: ")
	if frontCorrect && backCorrect {
		fmt.Printf("✓ PASS - Both sides regenerated correctly!\n\n")
		fmt.Printf("  ✓ Front audio contains: %s\n", ts.currentWord)
		fmt.Printf("  ✓ Back audio contains: %s\n", ts.currentTranslation)
	} else if !frontCorrect && !backCorrect {
		fmt.Printf("✗ FAIL - Both sides regenerated incorrectly!\n\n")
		if containsString(string(frontContent), "BACK_") {
			fmt.Printf("  ✗ Front audio contains BACK definition (wrong!)\n")
		}
		if containsString(string(backContent), "FRONT_") {
			fmt.Printf("  ✗ Back audio contains FRONT word (wrong!)\n")
		}
	} else if !frontCorrect {
		fmt.Printf("✗ FAIL - Front audio is wrong!\n\n")
		fmt.Printf("  ✗ Front audio: %s\n", string(frontContent))
	} else if !backCorrect {
		fmt.Printf("✗ FAIL - Back audio is wrong!\n\n")
		fmt.Printf("  ✗ Back audio: %s\n", string(backContent))
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && (
		containsSubstring(haystack, needle))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func centerText(text string, width int) string {
	padding := (width - len(text)) / 2
	left := ""
	for i := 0; i < padding; i++ {
		left += " "
	}
	right := ""
	for i := 0; i < width-len(text)-padding; i++ {
		right += " "
	}
	return left + text + right
}

func repeatStr(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

func main() {
	// Create temp directory for test
	tmpDir := "/tmp/totalrecall_test"
	os.MkdirAll(tmpDir, 0755)

	// Test case: котка == домашно животно
	state := &TestState{
		currentWord:        "котка",
		currentCardType:    "bg-bg",
		currentTranslation: "домашно животно",
		cardDir:            tmpDir,
	}

	state.TestSequence()

	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Printf("Test files created in: %s\n", tmpDir)
	fmt.Println(repeatStr("=", 80) + "\n")
}
