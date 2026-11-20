package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// secretValue builds a header value at runtime, so no secret-shaped literal is committed.
func secretValue() string {
	return strings.Join([]string{"Bearer", "x" + strings.Repeat("7", 12) + "-runtime"}, " ")
}

// fakeServer answers chat requests with the given replies in order and records headers.
func fakeServer(t *testing.T, replies ...string) (*httptest.Server, *[]http.Header) {
	t.Helper()
	var seen []http.Header
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Clone())
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		content := replies[min(i, len(replies)-1)]
		i++
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, part := range strings.SplitAfter(content, "\n") {
				chunk, _ := json.Marshal(map[string]any{"model": req.Model, "choices": []any{map[string]any{"delta": map[string]string{"content": part}}}})
				fmt.Fprintf(w, "data: %s\n\n", chunk)
			}
			fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":22}}\n\ndata: [DONE]\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   req.Model,
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}},
			"usage":   map[string]int{"prompt_tokens": 11, "completion_tokens": 22},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func endpointFor(srv *httptest.Server) Endpoint {
	return Endpoint{Label: "fake", BaseURL: srv.URL, Headers: http.Header{"Authorization": {secretValue()}}}
}

const goodReply = "Here you go.\n=== FILE: sample/orders/engine.go ===\n```go\npackage orders\n```\n=== END FILE ===\n"

func TestCompletionHappyPath(t *testing.T) {
	srv, seen := fakeServer(t, goodReply)
	r, err := Completion{Endpoint: endpointFor(srv)}.Call(context.Background(), Request{Role: "implementer", Model: "m1", Messages: []Message{{"user", "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "m1" || r.TokensIn != 11 || r.TokensOut != 22 || r.Source != "live" {
		t.Fatalf("reply = %+v", r)
	}
	if got := (*seen)[0].Get("Authorization"); got != secretValue() {
		t.Fatal("header not sent")
	}
	files, err := ParseFiles(r.Content)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "sample/orders/engine.go" || files[0].Content != "package orders\n" {
		t.Fatalf("files = %+v", files)
	}
}

func TestCompletionStream(t *testing.T) {
	srv, _ := fakeServer(t, goodReply)
	var out bytes.Buffer
	r, err := Completion{Endpoint: endpointFor(srv)}.Call(context.Background(), Request{Role: "implementer", Model: "m1", Stream: &out})
	if err != nil {
		t.Fatal(err)
	}
	if r.Content != goodReply || out.String() != goodReply || r.TokensOut != 22 {
		t.Fatalf("stream: content %q, printed %q, reply %+v", r.Content, out.String(), r)
	}
}

func TestCompletionTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	_, err := Completion{Endpoint: endpointFor(srv)}.Call(context.Background(), Request{Model: "m", Timeout: 50 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestCompletionErrorsNeverShowHeadersOrURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	ep := endpointFor(srv)
	_, err := Completion{Endpoint: ep}.Call(context.Background(), Request{Model: "m"})
	srv.Close()
	_, err2 := Completion{Endpoint: ep}.Call(context.Background(), Request{Model: "m"}) // connection refused
	for _, e := range []error{err, err2} {
		if e == nil || strings.Contains(e.Error(), secretValue()) || strings.Contains(e.Error(), srv.URL) {
			t.Fatalf("error leaks endpoint details or is nil: %v", e)
		}
	}
}

func TestParseFiles(t *testing.T) {
	tests := []struct {
		name, reply string
		wantErr     bool
		wantFiles   int
	}{
		{"plain", goodReply, false, 1},
		{"reasoning stripped", "<think>=== FILE: /etc/x ===\nno\n=== END FILE ===</think>" + goodReply, false, 1},
		{"opening think tag swallowed", "I think about it.</think>\n" + goodReply, false, 1},
		{"two files", goodReply + "=== FILE: sample/orders/orders.go ===\npackage orders\n=== END FILE ===\n", false, 2},
		{"no block", "I changed engine.go to allow partial fills.", true, 0},
		{"unterminated", "=== FILE: a.go ===\npackage a\n", true, 0},
		{"absolute path", "=== FILE: /etc/passwd ===\nx\n=== END FILE ===\n", true, 0},
		{"escape", "=== FILE: ../outside.go ===\nx\n=== END FILE ===\n", true, 0},
		{"sneaky escape", "=== FILE: sample/../../outside.go ===\nx\n=== END FILE ===\n", true, 0},
		{"git dir", "=== FILE: .git/config ===\nx\n=== END FILE ===\n", true, 0},
		{"duplicate", goodReply + goodReply, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ParseFiles(tt.reply)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrAgent) {
				t.Fatalf("parse error is not an agent error: %v", err)
			}
			if len(files) != tt.wantFiles {
				t.Fatalf("files = %d, want %d", len(files), tt.wantFiles)
			}
		})
	}
}

func TestWriteFilesFormatsGo(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFiles(dir, []File{{Path: "a/a.go", Content: "package a\nfunc  F( ) {}\n"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a", "a.go"))
	if string(data) != "package a\n\nfunc F() {}\n" {
		t.Fatalf("not gofmt-ed: %q", data)
	}
}

func TestParseHeadersAndResolve(t *testing.T) {
	v := secretValue()
	env := map[string]string{"U": "http://h:1/v1/", "H": "Authorization: " + v + `\nX-Tenant: t1`}
	ep, err := ResolveEndpoint("spark", "U", "H", func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if ep.BaseURL != "http://h:1/v1" || ep.Headers.Get("Authorization") != v || ep.Headers.Get("X-Tenant") != "t1" {
		t.Fatalf("endpoint = %+v", ep.BaseURL)
	}
	if _, err := ResolveEndpoint("spark", "MISSING", "H", func(k string) string { return env[k] }); err == nil {
		t.Fatal("missing URL accepted")
	}
	_, err = ParseHeaders("no colon " + v)
	if err == nil || strings.Contains(err.Error(), v) {
		t.Fatalf("bad header error leaks or is nil: %v", err)
	}
}

func TestRecordAndReplay(t *testing.T) {
	srv, _ := fakeServer(t, "first", "second")
	dir := t.TempDir()
	rec := &Recorder{Next: Completion{Endpoint: endpointFor(srv)}, Dir: dir, Endpoint: "fake"}
	reqs := []Request{
		{Role: "implementer", Model: "m1", Messages: []Message{{"user", "attempt 1"}}},
		{Role: "implementer", Model: "m1", Messages: []Message{{"user", "attempt 2"}}},
	}
	for _, r := range reqs {
		if _, err := rec.Call(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	// The recording holds no header value and no URL.
	paths, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, p := range paths {
		data, _ := os.ReadFile(p)
		if strings.Contains(string(data), secretValue()) || strings.Contains(string(data), srv.URL) {
			t.Fatalf("%s contains a header value or the endpoint URL", p)
		}
	}

	rep, err := LoadReplayer(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"first", "second"} {
		got, err := rep.Call(context.Background(), reqs[i])
		if err != nil || got.Content != want || got.Source != "replayed" {
			t.Fatalf("replay %d = %+v, %v", i, got, err)
		}
	}
	if _, err := rep.Call(context.Background(), reqs[0]); !errors.Is(err, ErrStale) {
		t.Fatalf("extra call: err = %v, want stale", err)
	}

	rep, _ = LoadReplayer(dir)
	changed := Request{Role: "implementer", Model: "m1", Messages: []Message{{"user", "a different prompt"}}}
	if _, err := rep.Call(context.Background(), changed); !errors.Is(err, ErrStale) {
		t.Fatalf("changed prompt: err = %v, want stale", err)
	}
}

func TestModelFor(t *testing.T) {
	r := RoleConfig{Model: "base", Ladder: []string{"small", "medium"}}
	for attempt, want := range map[int]string{1: "small", 2: "medium", 3: "medium"} {
		if got := r.ModelFor(attempt); got != want {
			t.Errorf("ModelFor(%d) = %s, want %s", attempt, got, want)
		}
	}
	if got := (RoleConfig{Model: "base"}).ModelFor(2); got != "base" {
		t.Errorf("no ladder: %s", got)
	}
}
