package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EricGrill/linear-scout/internal/model"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

type OpenAIOption func(*openai.ClientConfig)

func WithBaseURL(url string) OpenAIOption {
	return func(c *openai.ClientConfig) { c.BaseURL = url }
}

// WithHTTPClient overrides the HTTP client (e.g. to add retry/backoff).
func WithHTTPClient(hc *http.Client) OpenAIOption {
	return func(c *openai.ClientConfig) { c.HTTPClient = hc }
}

func NewOpenAI(apiKey, modelName string, opts ...OpenAIOption) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	for _, o := range opts {
		o(&cfg)
	}
	return &OpenAIProvider{client: openai.NewClientWithConfig(cfg), model: modelName}
}

type wireRec struct {
	Summary      string  `json:"summary"`
	WhyItMatters string  `json:"why_it_matters"`
	Confidence   float64 `json:"confidence"`
	Evidence     []struct {
		Ref string `json:"ref"`
		URL string `json:"url"`
	} `json:"evidence"`
	DraftTitle string `json:"draft_title"`
	DraftBody  string `json:"draft_body"`
}

func (p *OpenAIProvider) Recommend(ctx context.Context, req Request) ([]model.Recommendation, error) {
	prompt := AssemblePrompt(req)
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:          p.model,
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "Respond with a JSON object: {\"recommendations\":[...]}."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}
	var wire struct {
		Recommendations []wireRec `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &wire); err != nil {
		return nil, fmt.Errorf("parse openai json: %w", err)
	}
	out := make([]model.Recommendation, 0, len(wire.Recommendations))
	for _, w := range wire.Recommendations {
		links := make([]model.EvidenceLink, 0, len(w.Evidence))
		for _, e := range w.Evidence {
			links = append(links, model.EvidenceLink{Kind: "issue", Ref: e.Ref, URL: e.URL})
		}
		out = append(out, model.Recommendation{
			Summary: w.Summary, WhyItMatters: w.WhyItMatters,
			Confidence: model.Confidence(w.Confidence), Evidence: links,
			DraftTitle: w.DraftTitle, DraftBody: w.DraftBody,
		})
	}
	return out, nil
}
