// Package extractor based on algorithm N-gram
package extractor

import (
	"errors"
	"regexp"
	"strings"
)

var (
	handlingUnnecessaryCharactersRegex = regexp.MustCompile(`[^\p{L}\d\s\.\+#\-/]`)
	handlingSpacesRegex                = regexp.MustCompile(`\s+`)
)

// ExtractSkills returns a dictionary of found skills with the number of mentions, using N-gram algorithm.
//
// text - source text for analysis.
// whiteList - dictionary of allowed skills (the key is the skill, the value is ignored).
// maxNgram - maximum N-gram length (number of words in a phrase).
func ExtractSkills(text string, whiteList map[string]int, maxNgram int) (map[string]int, error) {
	if text == "" {
		return nil, errors.New("text cannot be empty")
	}
	if len(whiteList) == 0 {
		return nil, errors.New("whiteList cannot be empty")
	}
	if maxNgram <= 0 {
		return nil, errors.New("maxNgram must be positive")
	}

	text = strings.ToLower(text)

	preparedText := handlingUnnecessaryCharactersRegex.ReplaceAllString(text, " ")
	preparedText = handlingSpacesRegex.ReplaceAllString(preparedText, " ")
	preparedText = strings.TrimSpace(preparedText)

	words := strings.Fields(preparedText)

	result := make(map[string]int)
	n := len(words)
	for i := 0; i < n; i++ {
		for j := 1; j <= maxNgram && i+j <= n; j++ {
			ngram := strings.Join(words[i:i+j], " ")
			if strings.HasSuffix(ngram, ".") {
				ngram = ngram[:len(ngram)-1]
			}
			if _, ok := whiteList[ngram]; ok {
				result[ngram]++
			}
		}
	}

	return result, nil
}
