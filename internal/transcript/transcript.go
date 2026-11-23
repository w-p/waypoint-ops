// Package transcript reads per-tool session transcripts for what the session-events contract does
// not carry: whether a session is working or waiting for input, its latest activity, live token
// use and context size. Claude Code's JSONL transcripts are the only format read so far.
//
// It reports tool names and file base names only: never prompts, replies, code or commands.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/w-p/waypoint-ops/internal/events"
)

// FormatClaudeCode is the contract's name for Claude Code's transcript format.
const FormatClaudeCode = "claude-code-jsonl"

// Supported reports whether a transcript format can be read.
func Supported(format string) bool {
	return format == FormatClaudeCode
}

// Turn is whose move it is in a session.
type Turn int

// Turns.
const (
	// Unknown: nothing in the transcript says yet.
	Unknown Turn = iota
	// Working: the model is answering or a tool is running.
	Working
	// Idle: the turn ended and the session waits for input.
	Idle
)

// Spend is tokens used within one minute.
type Spend struct {
	At     time.Time
	Tokens int64
}

// Activity is what a transcript says about its session so far.
type Activity struct {
	// Last is the newest timestamped line.
	Last time.Time

	Turn      Turn
	TurnSince time.Time

	// Tool is the tool running while Working, and Target the base name of the file it works on.
	Tool   string
	Target string

	Model       string
	Tokens      events.Tokens
	Context     int64
	Compactions int
	Replies     int

	// Spend is token use per minute, oldest first.
	Spend []Spend

	// Subagents are the session's subagents, from their own transcripts beside the session's,
	// oldest first.
	Subagents []Subagent
}

// Subagent is one subagent as its transcript shows it.
type Subagent struct {
	ID      string
	Started time.Time
	Last    time.Time

	// Done is set once the subagent handed back its result or ended its turn.
	Done bool
}

// Reader follows one transcript file, reading only what was added since the last read.
type Reader struct {
	Path string
	act  Activity

	offset int64

	// subs follows each subagent transcript by file, rereading it only when its size changes.
	subs map[string]*subFile

	// seen holds the usage already counted for each reply, because Claude Code repeats a reply's
	// usage on every line it writes for that reply.
	seen map[string]events.Tokens
}

// NewReader makes a reader for path. A leading "~/" is expanded to the home folder.
func NewReader(path string) *Reader {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}
	return &Reader{Path: path, seen: map[string]events.Tokens{}, subs: map[string]*subFile{}}
}

// Activity returns what has been read so far, as a copy the reader will not change.
func (r *Reader) Activity() Activity {
	a := r.act
	a.Spend = append([]Spend(nil), r.act.Spend...)
	a.Subagents = append([]Subagent(nil), r.act.Subagents...)
	return a
}

// Read reads lines added since the last read, up to the first line written after until; that line
// and those after it wait for a later read. A file that shrank is read again from the start.
func (r *Reader) Read(until time.Time) error {
	err := r.read(until)
	r.readSubagents(until)
	return err
}

func (r *Reader) read(until time.Time) error {
	fh, err := os.Open(r.Path)
	if err != nil {
		return err
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		return err
	}
	if info.Size() < r.offset {
		r.offset = 0
		r.act = Activity{}
		r.seen = map[string]events.Tokens{}
	}
	if info.Size() == r.offset {
		return nil
	}
	if _, err := fh.Seek(r.offset, io.SeekStart); err != nil {
		return err
	}
	in := bufio.NewReaderSize(fh, 1<<20)
	for {
		line, err := in.ReadBytes('\n')
		if err == io.EOF {
			return nil // a partial last line waits for the writer to finish it
		}
		if err != nil {
			return err
		}
		at := stamp(line)
		if !at.IsZero() && at.After(until) {
			return nil
		}
		r.offset += int64(len(line))
		r.apply(line, at)
	}
}

var stampKey = []byte(`"timestamp":"`)

// stamp finds a line's timestamp without decoding the whole line.
func stamp(line []byte) time.Time {
	i := bytes.Index(line, stampKey)
	if i < 0 {
		return time.Time{}
	}
	rest := line[i+len(stampKey):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, string(rest[:j]))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Only these line types say anything about the turn, tokens or compactions; the rest are skipped
// before decoding.
var wanted = [][]byte{[]byte(`"type":"assistant"`), []byte(`"type":"user"`), []byte(`"type":"system"`)}

type entry struct {
	Type             string `json:"type"`
	Subtype          string `json:"subtype"`
	IsSidechain      bool   `json:"isSidechain"`
	IsMeta           bool   `json:"isMeta"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Message          *struct {
		ID         string          `json:"id"`
		Model      string          `json:"model"`
		StopReason string          `json:"stop_reason"`
		Content    json.RawMessage `json:"content"`
		Usage      *struct {
			Input      int64 `json:"input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
			Output     int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type block struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Input *struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
	} `json:"input"`
}

func (r *Reader) apply(line []byte, at time.Time) {
	if at.After(r.act.Last) {
		r.act.Last = at
	}
	interesting := false
	for _, w := range wanted {
		if bytes.Contains(line, w) {
			interesting = true
			break
		}
	}
	if !interesting {
		return
	}
	var e entry
	if json.Unmarshal(line, &e) != nil || e.IsSidechain {
		return
	}
	switch e.Type {
	case "system":
		switch e.Subtype {
		case "compact_boundary":
			r.act.Compactions++
		case "stop_hook_summary":
			r.turn(Idle, at, "", "")
		}
	case "assistant":
		r.assistant(e, at)
	case "user":
		if e.IsMeta || e.IsCompactSummary || e.Message == nil {
			return
		}
		blocks, text := content(e.Message.Content)
		if strings.HasPrefix(text, "[Request interrupted") {
			r.turn(Idle, at, "", "")
			return
		}
		for _, b := range blocks {
			if b.Type == "text" && strings.HasPrefix(b.Text, "[Request interrupted") {
				r.turn(Idle, at, "", "")
				return
			}
		}
		r.turn(Working, at, "", "")
	}
}

func (r *Reader) assistant(e entry, at time.Time) {
	m := e.Message
	if m == nil {
		return
	}
	if m.Model != "" && !strings.HasPrefix(m.Model, "<") {
		r.act.Model = m.Model
	}
	if u := m.Usage; u != nil {
		now := events.Tokens{Input: u.Input, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead, Output: u.Output}
		before, counted := r.seen[m.ID]
		if !counted {
			r.act.Replies++
		}
		delta := events.Tokens{
			Input:      max(0, now.Input-before.Input),
			CacheWrite: max(0, now.CacheWrite-before.CacheWrite),
			CacheRead:  max(0, now.CacheRead-before.CacheRead),
			Output:     max(0, now.Output-before.Output),
		}
		if len(r.seen) > 4096 {
			r.seen = map[string]events.Tokens{} // replies are written in order; old ones never return
		}
		r.seen[m.ID] = now
		r.act.Tokens.Input += delta.Input
		r.act.Tokens.CacheWrite += delta.CacheWrite
		r.act.Tokens.CacheRead += delta.CacheRead
		r.act.Tokens.Output += delta.Output
		r.spend(at, delta.Total())
		if c := now.Input + now.CacheWrite + now.CacheRead; c > 0 {
			r.act.Context = c
		}
	}
	blocks, _ := content(m.Content)
	for i := len(blocks) - 1; i >= 0; i-- {
		if b := blocks[i]; b.Type == "tool_use" {
			target := ""
			if in := b.Input; in != nil {
				for _, p := range []string{in.FilePath, in.NotebookPath, in.Path} {
					if p != "" {
						target = filepath.Base(p)
						break
					}
				}
			}
			r.turn(Working, at, b.Name, target)
			return
		}
	}
	if m.StopReason == "end_turn" {
		r.turn(Idle, at, "", "")
		return
	}
	r.turn(Working, at, "", "")
}

// turn records the turn state, noting when it changed.
func (r *Reader) turn(t Turn, at time.Time, tool, target string) {
	if t != r.act.Turn || tool != r.act.Tool {
		r.act.TurnSince = at
	}
	r.act.Turn, r.act.Tool, r.act.Target = t, tool, target
}

func (r *Reader) spend(at time.Time, n int64) {
	if n <= 0 || at.IsZero() {
		return
	}
	minute := at.Truncate(time.Minute)
	if k := len(r.act.Spend); k > 0 && r.act.Spend[k-1].At.Equal(minute) {
		r.act.Spend[k-1].Tokens += n
		return
	}
	r.act.Spend = append(r.act.Spend, Spend{minute, n})
}

// content decodes a message's content, which is either a string or a list of blocks.
func content(raw json.RawMessage) ([]block, string) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, ""
	}
	if raw[0] == '"' {
		var s string
		_ = json.Unmarshal(raw, &s)
		return nil, s
	}
	var blocks []block
	_ = json.Unmarshal(raw, &blocks)
	return blocks, ""
}

// subFile is what is known of one subagent transcript.
type subFile struct {
	size int64
	sub  Subagent
}

// readSubagents follows the subagent transcripts Claude Code writes beside the session's own, in
// a folder named after it: <session>/subagents/agent-<id>.jsonl. Only a file's first and last
// lines are read, and only when its size changed. Subagents started after until are left out.
func (r *Reader) readSubagents(until time.Time) {
	dir := filepath.Join(strings.TrimSuffix(r.Path, ".jsonl"), "subagents")
	files, _ := filepath.Glob(filepath.Join(dir, "agent-*.jsonl"))
	var out []Subagent
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		f := r.subs[path]
		if f == nil || f.size != info.Size() {
			sub, ok := readSubagent(path, info.Size())
			if !ok {
				continue
			}
			f = &subFile{size: info.Size(), sub: sub}
			r.subs[path] = f
		}
		if f.sub.Started.IsZero() || f.sub.Started.After(until) {
			continue
		}
		out = append(out, f.sub)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	r.act.Subagents = out
}

// tailSize is how much of a subagent transcript's end is read to find its last line.
const tailSize = 256 << 10

// readSubagent reads a subagent transcript's id, first timestamp, and how it ends.
func readSubagent(path string, size int64) (Subagent, bool) {
	fh, err := os.Open(path)
	if err != nil {
		return Subagent{}, false
	}
	defer fh.Close()
	sub := Subagent{ID: strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "agent-"), ".jsonl")}
	head, err := bufio.NewReaderSize(fh, 64<<10).ReadBytes('\n')
	if err != nil && len(head) == 0 {
		return Subagent{}, false
	}
	sub.Started = stamp(head)

	start := max(0, size-tailSize)
	tail := make([]byte, size-start)
	if _, err := fh.ReadAt(tail, start); err != nil && err != io.EOF {
		return Subagent{}, false
	}
	lines := bytes.Split(tail, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if i == 0 && start > 0 {
			break // cut off by the tail's start
		}
		if at := stamp(line); !at.IsZero() && at.After(sub.Last) {
			sub.Last = at
		}
		if done, known := ended(line); known {
			sub.Done = done
			break
		}
	}
	if sub.Last.IsZero() {
		sub.Last = sub.Started
	}
	return sub, true
}

// ended reads one subagent transcript line: known is false for lines that say nothing about the
// turn (request records, attachments); done is true for the hand-back (a tool result that ends
// the turn) or a reply that ends the turn.
func ended(line []byte) (done, known bool) {
	if !bytes.Contains(line, []byte(`"type":"assistant"`)) && !bytes.Contains(line, []byte(`"type":"user"`)) {
		return false, false
	}
	var e struct {
		Type         string `json:"type"`
		ToolEndsTurn bool   `json:"toolEndsTurn"`
		Message      *struct {
			StopReason string `json:"stop_reason"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &e) != nil {
		return false, false
	}
	switch e.Type {
	case "user":
		return e.ToolEndsTurn, true
	case "assistant":
		if e.Message == nil {
			return false, true
		}
		return e.Message.StopReason == "end_turn" || e.Message.StopReason == "stop_sequence", true
	}
	return false, false
}
