package extractor_test

import (
	"psa/internal/usecase/extractor"
	"testing"
)

func TestExtractSkills(t *testing.T) {
	ext := extractor.New()

	tests := []struct {
		name      string
		text      string
		whiteList map[string]int
		maxNgram  int
		expected  map[string]int
		wantError bool
	}{
		{
			name:      "empty text",
			text:      "",
			whiteList: map[string]int{"go": 1},
			maxNgram:  3,
			wantError: true,
		},
		{
			name:      "empty whitelist",
			text:      "some text",
			whiteList: map[string]int{},
			maxNgram:  3,
			wantError: true,
		},
		{
			name:      "invalid maxNgram",
			text:      "some text",
			whiteList: map[string]int{"go": 1},
			maxNgram:  0,
			wantError: true,
		},
		{
			name:      "basic extraction",
			text:      "We need a Go developer with Python experience",
			whiteList: map[string]int{"go": 1, "python": 1, "ruby": 1},
			maxNgram:  3,
			expected:  map[string]int{"go": 1, "python": 1},
		},
		{
			name: "ngram boundaries",
			text: "software engineer puk senior developer",
			whiteList: map[string]int{"software": 1, "engineer": 1, "software engineer": 1,
				"senior developer": 1},
			maxNgram: 3,
			expected: map[string]int{"software": 1, "engineer": 1, "software engineer": 1,
				"senior developer": 1},
		},
		{
			name:      "case insensitivity",
			text:      "GO Python JAVA",
			whiteList: map[string]int{"go": 1, "python": 1, "java": 1},
			maxNgram:  3,
			expected:  map[string]int{"go": 1, "python": 1, "java": 1},
		},
		{
			name:      "punctuation handling",
			text:      "We need: Go, Python, and Java developers!",
			whiteList: map[string]int{"go": 1, "python": 1, "java": 1},
			maxNgram:  3,
			expected:  map[string]int{"go": 1, "python": 1, "java": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ext.ExtractSkills(tt.text, tt.whiteList, tt.maxNgram)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)

				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d skills, got %d", len(tt.expected), len(result))

				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("For skill %s, expected count %d, got %d", k, v, result[k])
				}
			}
		})
	}
}
