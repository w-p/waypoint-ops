package transcript

import (
	"fmt"
	"strings"
	"testing"
)

func tool(name, input string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-11-21T14:00:06.000Z","message":{"id":"w","stop_reason":"tool_use","content":[{"type":"tool_use","name":%q,"input":%s}]}}`, name, input)
}

func bash(cmd string) string {
	return tool("Bash", fmt.Sprintf(`{"command":%q}`, cmd))
}

func TestIsWork(t *testing.T) {
	tests := []struct {
		line string
		work bool
	}{
		{tool("Edit", `{"file_path":"/x/view.go"}`), true},
		{tool("Write", `{"file_path":"/x/.waypoint/memory/2026-11-23-a.md"}`), false},
		{tool("NotebookEdit", `{"notebook_path":"/x/a.ipynb"}`), true},
		{tool("Agent", `{}`), true},
		{tool("Read", `{"file_path":"/x/view.go"}`), false},
		{bash("git commit -qm x"), true},
		{bash("git -C /x commit -m x"), true},
		{bash("git status"), false},
		{bash("go test ./... 2>&1 | tail"), false},
		{bash("make > /dev/null"), false},
		{bash("echo hi > out.txt"), true},
		{bash("echo hi >> \"$f\""), true},
		{bash("sed -i s/a/b/ x"), true},
		{bash("python3 - <<'EOF'\nx\nEOF"), true},
		{bash("cat <<EOF | tee x"), true},
		{bash("echo note >> .waypoint/memory/2026-11-23-a.md"), false},
		{bash("ls &> log"), false},
	}
	for _, tt := range tests {
		r := NewReader(write(t, tt.line))
		if err := r.Read(at("2026-11-22T00:00:00Z")); err != nil {
			t.Fatal(err)
		}
		if got := r.Activity().Work == 1; got != tt.work {
			t.Errorf("%s: work %v, want %v", tt.line, got, tt.work)
		}
	}
}

func TestWorkSummarized(t *testing.T) {
	edit := tool("Edit", `{"file_path":"/x/view.go"}`)
	lines := []string{prompt}
	lines = append(lines, strings.Split(strings.Repeat(edit+"\n", 3), "\n")[:3]...)
	lines = append(lines, compacted, edit, compacted, edit, edit)
	r := NewReader(write(t, lines...))
	if err := r.Read(at("2026-11-22T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	a := r.Activity()
	if a.Compactions != 2 || a.WorkSummarized != 4 || a.Work != 2 {
		t.Errorf("compactions %d, summarized %d, work %d; want 2, 4, 2", a.Compactions, a.WorkSummarized, a.Work)
	}
	if !a.LastCompaction.Equal(at("2026-11-21T14:02:00Z")) || !a.First.Equal(at("2026-11-21T14:00:00Z")) {
		t.Errorf("last compaction %s, first %s", a.LastCompaction, a.First)
	}
}
