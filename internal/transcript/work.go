package transcript

import (
	"regexp"
	"strings"
)

// The pilot's events hook counts a session's work this way (its `note_tool`), and its fatigue
// reading counts the work compactions summarized away. Reading the same transcript the same way
// keeps a worked-out reading close to the hook's (`design/session-condition.md`).
var (
	memoryFile     = regexp.MustCompile(`(?:^|/)\.waypoint/memory/[^/\s'"]+\.md`)
	memoryRedirect = regexp.MustCompile(`(?:>>?|\btee\b)[^|;&]*\.waypoint/memory/`)
	commit         = regexp.MustCompile(`\bgit (?:-C \S+ )?commit\b`)
	shellEdit      = regexp.MustCompile(`\bsed -i|\bperl -pi|\btee\b|\b(?:python3?|node|ruby|perl)\b[^|;&]*<<`)
)

// isWork reports whether a tool call is a work item: an edit outside the memory folder, a subagent
// started, or a shell command that commits or writes files. Writing the memory file is saving,
// not work.
func isWork(b block) bool {
	switch b.Name {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		target := ""
		if in := b.Input; in != nil {
			target = in.FilePath
			if target == "" {
				target = in.NotebookPath
			}
		}
		return !memoryFile.MatchString(target)
	case "Agent", "Task":
		return true
	case "Bash":
		if b.Input == nil {
			return false
		}
		cmd := b.Input.Command
		if memoryRedirect.MatchString(cmd) {
			return false
		}
		return commit.MatchString(cmd) || shellEdit.MatchString(cmd) || redirectsToFile(cmd)
	}
	return false
}

// redirectsToFile reports whether a command sends output into a file with > or >>: not 2>, &>,
// a second > of >>, or /dev/null.
func redirectsToFile(cmd string) bool {
	for i := 0; i < len(cmd); i++ {
		if cmd[i] != '>' {
			continue
		}
		if i > 0 {
			if c := cmd[i-1]; c >= '0' && c <= '9' || c == '&' || c == '>' {
				continue
			}
		}
		rest := cmd[i+1:]
		rest = strings.TrimPrefix(rest, ">")
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" || strings.HasPrefix(rest, "/dev/null") {
			continue
		}
		if c := rest[0]; isWord(c) || strings.IndexByte(`"'$./~-`, c) >= 0 {
			return true
		}
	}
	return false
}

func isWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
