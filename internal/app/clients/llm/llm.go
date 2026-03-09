/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package llm

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	client          *genai.Client
	model           *genai.GenerativeModel
	enableTranslate bool
}

func NewGeminiClient(ctx context.Context, apiKey string, enableTranslate bool) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	// gemini-2.5-flash работает очень быстро — идеально для чат-ботов
	model := client.GenerativeModel("gemini-2.5-flash")
	return &GeminiClient{client: client, model: model, enableTranslate: enableTranslate}, nil
}

// Translate переводит текст. Если текст пустой или произошла ошибка, возвращает оригинал.
func (g *GeminiClient) Translate(ctx context.Context, text, targetLang string) string {
	if !g.enableTranslate {
		return text
	}

	if text == "" {
		return ""
	}

	prompt := fmt.Sprintf("Translate the following text to language code '%s'. Return ONLY the translated text without any quotes, markdown formatting, or comments. If the text is already in '%s', just return it as is:\n\n%s", targetLang, targetLang, text)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Failed to translate text: %v\n", err)
		return text // Fallback: если LLM упала, шлем оригинал
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		part := resp.Candidates[0].Content.Parts[0]
		if str, ok := part.(genai.Text); ok {
			return strings.TrimSpace(string(str))
		}
	}
	return text
}

func (g *GeminiClient) Close() {
	g.client.Close()
}
