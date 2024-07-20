package lib

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bregydoc/gtranslate"
)

func Translate(text string) (result *string, err error) {
	const chunkSize = 2000
	var resultBuilder strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := string(runes[i:end])
		translated, translateErr := translateWithRetry(chunk, 3)
		if translateErr != nil {
			log.Printf("Error translating chunk: %v", translateErr)
			resultBuilder.WriteString(chunk)
		} else {
			resultBuilder.WriteString(translated)
		}
	}

	translatedResult := resultBuilder.String()
	return &translatedResult, nil
}

func translateWithTimeout(chunk string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		translated, err := gtranslate.TranslateWithParams(
			chunk,
			gtranslate.TranslationParams{
				From: "zh",
				To:   "en",
			},
		)
		if err != nil {
			errChan <- err
		} else {
			resultChan <- translated
		}
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("translation timed out")
	}
}

func translateWithRetry(chunk string, maxRetries int) (string, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		translated, err := translateWithTimeout(chunk)
		if err == nil {
			return translated, nil
		}
		log.Printf("Translation attempt %d failed: %v", attempt+1, err)
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return chunk, fmt.Errorf("all translation attempts failed")
}
