package lib

import (
	"fmt"
	"log"
	"strings"

	"github.com/bregydoc/gtranslate"
)

func Translate(text string) (result *string, err error) {

	const chunkSize = 1500
	var resultBuilder strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := string(runes[i:end])
		translated, translateErr := gtranslate.TranslateWithParams(
			chunk,
			gtranslate.TranslationParams{
				From: "zh",
				To:   "en",
			},
		)
		if translateErr != nil {
			return nil, fmt.Errorf("translating chunk: %w", translateErr)
		}

		resultBuilder.WriteString(translated)
	}
	log.Printf("Translated %d characters", len(runes))

	translatedResult := resultBuilder.String()
	return &translatedResult, nil
}
