package controller

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"Hello World", "hello-world"},
		{"HELLO WORLD", "hello-world"},
		{"Hello!!!World", "hello-world"},
		{"Hello   World", "hello-world"},
		{"--Hello--World--", "hello-world"},
		{"123 Test", "123-test"},
		{"Test-123", "test-123"},
		{"", ""},
		{"!!!", ""},
		{"---", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEncodeMessage_Empty(t *testing.T) {
	result := encodeMessage("")
	if result != nil {
		t.Errorf("encodeMessage('') = %v, want nil", result)
	}
}

func TestEncodeMessage_SingleWord(t *testing.T) {
	result := encodeMessage("Hello")

	if len(result) != 3 {
		t.Errorf("expected 3 pods (header, message, footer), got %d", len(result))
	}

	if result[0] != headerPodName {
		t.Errorf("first pod should be header, got %q", result[0])
	}

	if result[1] != "01-hello" {
		t.Errorf("second pod should be '01-hello', got %q", result[1])
	}

	if result[2] != footerPodName {
		t.Errorf("last pod should be footer, got %q", result[2])
	}
}

func TestEncodeMessage_MultipleWords(t *testing.T) {
	result := encodeMessage("Hello World")

	if len(result) != 3 {
		t.Errorf("expected 3 pods, got %d", len(result))
	}

	if result[1] != "01-hello-world" {
		t.Errorf("expected '01-hello-world', got %q", result[1])
	}
}

func TestEncodeMessage_MultiLine(t *testing.T) {
	result := encodeMessage("Hello\nWorld")

	if len(result) != 4 {
		t.Errorf("expected 4 pods (header, 2 lines, footer), got %d: %v", len(result), result)
	}

	if result[1] != "01-hello" {
		t.Errorf("line 0 should be '01-hello', got %q", result[1])
	}

	if result[2] != "02-world" {
		t.Errorf("line 1 should be '02-world', got %q", result[2])
	}
}

func TestEncodeMessage_EscapedNewlines(t *testing.T) {
	result := encodeMessage("Hello\\nWorld")

	if len(result) != 4 {
		t.Errorf("expected 4 pods, got %d", len(result))
	}

	if result[1] != "01-hello" {
		t.Errorf("line 0 should be '01-hello', got %q", result[1])
	}

	if result[2] != "02-world" {
		t.Errorf("line 1 should be '02-world', got %q", result[2])
	}
}

func TestEncodeMessage_LongLine(t *testing.T) {
	// A very long line should be split into multiple pods
	longLine := strings.Repeat("word ", 20) // 100 chars
	result := encodeMessage(longLine)

	// Should have header + multiple message pods + footer
	if len(result) < 4 {
		t.Errorf("expected at least 4 pods for long line, got %d", len(result))
	}

	// All message pods should be under maxPodNameLen
	for i := 1; i < len(result)-1; i++ {
		if len(result[i]) > maxPodNameLen {
			t.Errorf("pod %d exceeds max length: %q (%d chars)", i, result[i], len(result[i]))
		}
	}
}

func TestEncodeMessage_VeryLongMessage(t *testing.T) {
	// Bee movie script length test - 1000+ words
	words := make([]string, 1000)
	for i := range words {
		words[i] = "bee"
	}
	longMessage := strings.Join(words, " ")

	result := encodeMessage(longMessage)

	// Should have header + many message pods + footer
	if len(result) < 50 {
		t.Errorf("expected many pods for long message, got %d", len(result))
	}

	// First and last should be header/footer
	if result[0] != headerPodName {
		t.Errorf("first pod should be header")
	}
	if result[len(result)-1] != footerPodName {
		t.Errorf("last pod should be footer")
	}

	// All message pods should be valid
	for i := 1; i < len(result)-1; i++ {
		if len(result[i]) > maxPodNameLen {
			t.Errorf("pod %d exceeds max length: %d chars", i, len(result[i]))
		}
	}
}

func TestEncodeMessage_SpecialCharacters(t *testing.T) {
	result := encodeMessage("Hello!!!World???123")

	if result[1] != "01-hello-world-123" {
		t.Errorf("special chars should be stripped, got %q", result[1])
	}
}

func TestEncodeMessage_UnicodeStripped(t *testing.T) {
	result := encodeMessage("Hello 🎉 World")

	// Emoji should be stripped
	if result[1] != "01-hello-world" {
		t.Errorf("emoji should be stripped, got %q", result[1])
	}
}

func TestEncodeMessage_PodNamesAreValidDNS(t *testing.T) {
	// Generate a complex message
	message := "Hello World!!! This is a TEST\nWith newlines\nAnd MORE text!!!"
	result := encodeMessage(message)

	for i, podName := range result {
		// DNS label rules:
		// - lowercase alphanumeric and dashes
		// - must start and end with alphanumeric
		// - max 63 chars (we use 58 for k9s)

		if len(podName) > 63 {
			t.Errorf("pod %d exceeds DNS label max: %q", i, podName)
		}

		if len(podName) > 0 {
			first := podName[0]
			last := podName[len(podName)-1]

			if !((first >= '0' && first <= '9') || (first >= 'a' && first <= 'z')) {
				t.Errorf("pod %d starts with invalid char: %q", i, podName)
			}

			if !((last >= '0' && last <= '9') || (last >= 'a' && last <= 'z')) {
				t.Errorf("pod %d ends with invalid char: %q", i, podName)
			}
		}
	}
}
