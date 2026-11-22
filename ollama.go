package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	BaseURL    string
	EmbedModel string
	GenModel   string
	Client     *http.Client
}

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
}

type GenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Stream  bool           `json:"stream"`
	Think   *bool          `json:"think,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewOllamaClient(baseURL, embedModel, genModel string) *OllamaClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
	}
	return &OllamaClient{
		BaseURL:    baseURL,
		EmbedModel: embedModel,
		GenModel:   genModel,
		Client:     &http.Client{Timeout: 120 * time.Second, Transport: transport},
	}
}

func (o *OllamaClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := EmbeddingRequest{
		Model:  o.EmbedModel,
		Prompt: text,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := o.BaseURL + "/api/embeddings"
	resp, err := o.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error llamando a Ollama embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error Ollama (%d): %s", resp.StatusCode, string(body))
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

func (o *OllamaClient) GenerateCompletion(ctx context.Context, prompt, systemPrompt string, think bool) (string, error) {
	reqBody := GenerateRequest{
		Model:  o.GenModel,
		Prompt: prompt,
		System: systemPrompt,
		Stream: false,
		Think:  &think,
		Options: map[string]any{
			"temperature": 0.1,
			"top_p":       0.9,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := o.BaseURL + "/api/generate"
	resp, err := o.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error llamando a Ollama generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error Ollama (%d): %s", resp.StatusCode, string(body))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	response := strings.TrimSpace(result.Response)
	if !think {
		if idx := strings.Index(response, "</think>"); idx != -1 {
			response = strings.TrimSpace(response[idx+len("</think>"):])
		}
	}

	return response, nil
}

func (o *OllamaClient) GenerateStream(ctx context.Context, prompt, systemPrompt string, think bool, onChunk func(token string) error) error {
	reqBody := GenerateRequest{
		Model:  o.GenModel,
		Prompt: prompt,
		System: systemPrompt,
		Stream: true,
		Think:  &think,
		Options: map[string]any{
			"temperature": 0.1,
			"top_p":       0.9,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := o.BaseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("error llamando a Ollama stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error Ollama (%d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	inThink := false
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var chunk struct {
				Response string `json:"response"`
				Done     bool   `json:"done"`
			}
			if jsonErr := json.Unmarshal(line, &chunk); jsonErr == nil {
				token := chunk.Response
				if strings.Contains(token, "<think>") {
					inThink = true
				}
				if inThink {
					if idx := strings.Index(token, "</think>"); idx != -1 {
						inThink = false
						token = token[idx+len("</think>"):]
					} else {
						token = ""
					}
				}
				if token != "" {
					if err := onChunk(token); err != nil {
						return err
					}
				}
				if chunk.Done {
					break
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}
