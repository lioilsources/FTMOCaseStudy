package agent

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

// Completion calls an OpenAI-compatible chat endpoint. It does not depend on JSON mode
// or tool calling, because local servers differ.
type Completion struct {
	Endpoint Endpoint
	Client   *http.Client
}

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions map[string]any `json:"stream_options,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Temperature   *float64       `json:"temperature,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

// Call sends one chat request. Errors name the endpoint label, never its URL or headers.
func (c Completion) Call(ctx context.Context, req Request) (Reply, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	body := chatRequest{Model: req.Model, Messages: req.Messages}
	if req.Stream != nil {
		body.Stream = true
		body.StreamOptions = map[string]any{"include_usage": true}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Reply{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return Reply{}, fmt.Errorf("endpoint %s: bad request", c.Endpoint.Label)
	}
	hreq.Header = c.Endpoint.Headers.Clone()
	if hreq.Header == nil {
		hreq.Header = http.Header{}
	}
	hreq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.client().Do(hreq)
	if err != nil {
		if ctx.Err() != nil {
			return Reply{}, fmt.Errorf("endpoint %s: %w", c.Endpoint.Label, ctx.Err())
		}
		return Reply{}, fmt.Errorf("endpoint %s: request failed", c.Endpoint.Label)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return Reply{}, fmt.Errorf("endpoint %s: HTTP %d: %s", c.Endpoint.Label, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var r Reply
	if req.Stream != nil {
		r, err = readStream(resp.Body, req.Stream)
	} else {
		r, err = readWhole(resp.Body)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Reply{}, fmt.Errorf("endpoint %s: %w", c.Endpoint.Label, ctx.Err())
		}
		return Reply{}, fmt.Errorf("endpoint %s: %w", c.Endpoint.Label, err)
	}
	if r.Model == "" {
		r.Model = req.Model
	}
	r.Duration, r.Source = time.Since(start), "live"
	return r, nil
}

func (c Completion) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func readWhole(body io.Reader) (Reply, error) {
	var cr chatResponse
	if err := json.NewDecoder(body).Decode(&cr); err != nil {
		return Reply{}, fmt.Errorf("decode reply: %w", err)
	}
	if len(cr.Choices) == 0 {
		return Reply{}, fmt.Errorf("reply has no choices")
	}
	r := Reply{Model: cr.Model, Content: cr.Choices[0].Message.Content}
	if cr.Usage != nil {
		r.TokensIn, r.TokensOut = cr.Usage.PromptTokens, cr.Usage.CompletionTokens
	}
	return r, nil
}

// readStream reads server-sent events and copies content deltas to w as they arrive.
func readStream(body io.Reader, w io.Writer) (Reply, error) {
	var r Reply
	var content strings.Builder
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var cr chatResponse
		if err := json.Unmarshal([]byte(data), &cr); err != nil {
			return Reply{}, fmt.Errorf("decode stream chunk: %w", err)
		}
		if cr.Model != "" {
			r.Model = cr.Model
		}
		for _, ch := range cr.Choices {
			content.WriteString(ch.Delta.Content)
			_, _ = io.WriteString(w, ch.Delta.Content)
		}
		if cr.Usage != nil {
			r.TokensIn, r.TokensOut = cr.Usage.PromptTokens, cr.Usage.CompletionTokens
		}
	}
	if err := sc.Err(); err != nil {
		return Reply{}, err
	}
	r.Content = content.String()
	return r, nil
}
