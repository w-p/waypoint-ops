package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic lines in Claude Code's shape, cut down to the fields the reader uses.
const (
	prompt     = `{"type":"user","timestamp":"2026-11-21T14:00:00.000Z","message":{"role":"user","content":"look at the board"}}`
	thinking   = `{"type":"assistant","timestamp":"2026-11-21T14:00:05.000Z","message":{"id":"m1","model":"claude-opus-5-5","stop_reason":null,"content":[{"type":"thinking","thinking":""}],"usage":{"input_tokens":10,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000,"output_tokens":5}}}`
	toolCall   = `{"type":"assistant","timestamp":"2026-11-21T14:00:06.000Z","message":{"id":"m1","model":"claude-opus-5-5","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/x/internal/ui/view.go"}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000,"output_tokens":80}}}`
	toolResult = `{"type":"user","timestamp":"2026-11-21T14:00:07.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`
	reply      = `{"type":"assistant","timestamp":"2026-11-21T14:01:30.000Z","message":{"id":"m2","model":"claude-opus-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":20,"cache_creation_input_tokens":200,"cache_read_input_tokens":6000,"output_tokens":40}}}`
	stopHook   = `{"type":"system","subtype":"stop_hook_summary","timestamp":"2026-11-21T14:01:31.000Z"}`
	compacted  = `{"type":"system","subtype":"compact_boundary","timestamp":"2026-11-21T14:02:00.000Z"}`
	interrupt  = `{"type":"user","timestamp":"2026-11-21T14:03:00.000Z","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user]"}]}}`
	sidechain  = `{"type":"assistant","isSidechain":true,"timestamp":"2026-11-21T14:00:08.000Z","message":{"id":"s1","model":"claude-haiku-5-5","stop_reason":"end_turn","content":[],"usage":{"input_tokens":999,"output_tokens":999}}}`
	attachment = `{"type":"attachment","timestamp":"2026-11-21T14:00:09.000Z","attachment":{"type":"x"}}`
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func write(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRead(t *testing.T) {
	end := at("2026-11-21T15:00:00Z")
	tests := []struct {
		name    string
		lines   []string
		until   time.Time
		turn    Turn
		tool    string
		target  string
		tokens  int64
		context int64
		replies int
		compact int
		last    string
	}{
		{"prompt means working", []string{prompt}, end, Working, "", "", 0, 0, 0, 0, "2026-11-21T14:00:00Z"},
		{"tool call names the tool and file", []string{prompt, thinking, toolCall}, end, Working, "Edit", "view.go", 6090, 6010, 1, 0, "2026-11-21T14:00:06Z"},
		{"tool result is still working", []string{prompt, thinking, toolCall, toolResult}, end, Working, "", "", 6090, 6010, 1, 0, "2026-11-21T14:00:07Z"},
		{"end of turn waits for input", []string{prompt, thinking, toolCall, toolResult, reply}, end, Idle, "", "", 12350, 6220, 2, 0, "2026-11-21T14:01:30Z"},
		{"stop hook waits for input", []string{prompt, stopHook}, end, Idle, "", "", 0, 0, 0, 0, "2026-11-21T14:01:31Z"},
		{"compaction counted", []string{prompt, compacted}, end, Working, "", "", 0, 0, 0, 1, "2026-11-21T14:02:00Z"},
		{"interrupt waits for input", []string{prompt, thinking, interrupt}, end, Idle, "", "", 6015, 6010, 1, 0, "2026-11-21T14:03:00Z"},
		{"sidechain ignored", []string{prompt, sidechain}, end, Working, "", "", 0, 0, 0, 0, "2026-11-21T14:00:08Z"},
		{"skipped lines still move last", []string{prompt, attachment}, end, Working, "", "", 0, 0, 0, 0, "2026-11-21T14:00:09Z"},
		{"lines after until wait", []string{prompt, thinking, toolCall, toolResult, reply}, at("2026-11-21T14:00:06.5Z"), Working, "Edit", "view.go", 6090, 6010, 1, 0, "2026-11-21T14:00:06Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader(write(t, tt.lines...))
			if err := r.Read(tt.until); err != nil {
				t.Fatal(err)
			}
			a := r.Activity()
			if a.Turn != tt.turn || a.Tool != tt.tool || a.Target != tt.target {
				t.Errorf("turn = %v %q %q, want %v %q %q", a.Turn, a.Tool, a.Target, tt.turn, tt.tool, tt.target)
			}
			if a.Tokens.Total() != tt.tokens || a.Context != tt.context || a.Replies != tt.replies {
				t.Errorf("tokens %d context %d replies %d, want %d %d %d", a.Tokens.Total(), a.Context, a.Replies, tt.tokens, tt.context, tt.replies)
			}
			if a.Compactions != tt.compact {
				t.Errorf("compactions = %d, want %d", a.Compactions, tt.compact)
			}
			if !a.Last.Equal(at(tt.last)) {
				t.Errorf("last = %v, want %v", a.Last, tt.last)
			}
		})
	}
}

func TestReadIncremental(t *testing.T) {
	path := write(t, prompt, thinking)
	r := NewReader(path)
	end := at("2026-11-21T15:00:00Z")
	if err := r.Read(end); err != nil {
		t.Fatal(err)
	}
	// A half-written line waits; the rest of the reply is counted once.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(toolCall[:40])
	if err := r.Read(end); err != nil {
		t.Fatal(err)
	}
	if r.Activity().Tool != "" {
		t.Fatalf("partial line was read")
	}
	f.WriteString(toolCall[40:] + "\n")
	f.Close()
	if err := r.Read(end); err != nil {
		t.Fatal(err)
	}
	a := r.Activity()
	if a.Tool != "Edit" || a.Tokens.Total() != 6090 || a.Replies != 1 {
		t.Errorf("after append: tool %q tokens %d replies %d", a.Tool, a.Tokens.Total(), a.Replies)
	}
	if len(a.Spend) != 1 || a.Spend[0].Tokens != 6090 {
		t.Errorf("spend = %v, want one minute of 6090", a.Spend)
	}
}

// TestRealTranscript times a read of a real transcript named by WAYPOINT_TRANSCRIPT. Real data
// stays out of the repository, so it is skipped unless set.
func TestRealTranscript(t *testing.T) {
	path := os.Getenv("WAYPOINT_TRANSCRIPT")
	if path == "" {
		t.Skip("set WAYPOINT_TRANSCRIPT to a transcript path")
	}
	r := NewReader(path)
	began := time.Now()
	if err := r.Read(time.Now()); err != nil {
		t.Fatal(err)
	}
	a := r.Activity()
	t.Logf("read in %v: turn %v tool %q last %v model %s tokens %d context %d replies %d compactions %d minutes %d",
		time.Since(began), a.Turn, a.Tool, a.Last, a.Model, a.Tokens.Total(), a.Context, a.Replies, a.Compactions, len(a.Spend))
}
