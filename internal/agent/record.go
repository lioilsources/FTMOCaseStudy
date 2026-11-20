package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Call is one recorded model call. It holds no endpoint URL and no header.
type Call struct {
	Seq          int       `json:"seq"`
	Role         string    `json:"role"`
	Model        string    `json:"model"`
	Endpoint     string    `json:"endpoint"` // the label only
	PromptSHA256 string    `json:"prompt_sha256"`
	Messages     []Message `json:"messages"`
	Reply        Reply     `json:"reply"`
	Error        string    `json:"error,omitempty"`
	Seconds      float64   `json:"seconds"`
}

// Recorder passes calls through to a live caller and stores each one in Dir.
type Recorder struct {
	Next     Caller
	Dir      string
	Endpoint string // label for the record
	mu       sync.Mutex
	seq      int
}

// Call implements Caller.
func (r *Recorder) Call(ctx context.Context, req Request) (Reply, error) {
	reply, err := r.Next.Call(ctx, req)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	c := Call{Seq: r.seq, Role: req.Role, Model: req.Model, Endpoint: r.Endpoint,
		PromptSHA256: req.PromptHash(), Messages: req.Messages, Reply: reply, Seconds: reply.Duration.Seconds()}
	if err != nil {
		c.Error = err.Error()
	}
	if werr := writeCall(r.Dir, c); werr != nil {
		return reply, errors.Join(err, werr)
	}
	return reply, err
}

func writeCall(dir string, c Call) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("call-%03d.json", c.Seq)), append(data, '\n'), 0o644)
}

// ErrStale is returned when a replayed call does not match the recording.
var ErrStale = errors.New("recording is stale")

// Replayer serves calls from a recording by sequence and prompt hash.
type Replayer struct {
	calls []Call
	mu    sync.Mutex
	next  int
}

// LoadReplayer reads every call-NNN.json in dir.
func LoadReplayer(dir string) (*Replayer, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "call-*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no recorded calls in %s", dir)
	}
	sort.Strings(paths)
	r := &Replayer{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var c Call
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		r.calls = append(r.calls, c)
	}
	return r, nil
}

// Call implements Caller.
func (r *Replayer) Call(_ context.Context, req Request) (Reply, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.calls) {
		return Reply{}, fmt.Errorf("%w: call %d was not recorded", ErrStale, r.next+1)
	}
	c := r.calls[r.next]
	r.next++
	if c.Role != req.Role || c.PromptSHA256 != req.PromptHash() {
		return Reply{}, fmt.Errorf("%w: call %d (%s) has a different prompt than recorded", ErrStale, c.Seq, req.Role)
	}
	reply := c.Reply
	reply.Source = "replayed"
	if req.Stream != nil {
		_, _ = req.Stream.Write([]byte(reply.Content))
	}
	if c.Error != "" {
		return reply, errors.New(c.Error)
	}
	return reply, nil
}

// Remaining is the number of recorded calls not served yet.
func (r *Replayer) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls) - r.next
}
