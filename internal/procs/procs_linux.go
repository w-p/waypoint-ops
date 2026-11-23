package procs

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ticks is the kernel's clock tick rate for process times. It is 100 on every Linux build this
// app runs on; reading it properly needs sysconf, which needs cgo.
const ticks = 100

var pageSize = uint64(os.Getpagesize())

// list reads every process's parent, CPU time, memory and start from /proc in one pass.
func list() ([]proc, error) {
	boot := bootTime()
	dir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	out := make([]proc, 0, len(names))
	for _, name := range names {
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile("/proc/" + name + "/stat")
		if err != nil {
			continue // gone since the folder was read
		}
		if p, ok := parseStat(data, boot); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// parseStat reads one /proc/<pid>/stat line. The command name is in parentheses and may hold
// spaces or parentheses itself, so fields are counted from the last ')'.
func parseStat(data []byte, boot time.Time) (proc, bool) {
	open := bytes.IndexByte(data, '(')
	end := bytes.LastIndexByte(data, ')')
	if open < 1 || end < open {
		return proc{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data[:open])))
	if err != nil {
		return proc{}, false
	}
	// After ')': state(3) ppid(4) ... utime(14) stime(15) ... starttime(22) vsize(23) rss(24).
	f := strings.Fields(string(data[end+1:]))
	if len(f) < 22 {
		return proc{}, false
	}
	num := func(i int) uint64 { // field number as man 5 proc counts them
		n, _ := strconv.ParseUint(f[i-3], 10, 64)
		return n
	}
	p := proc{
		pid:  int32(pid),
		ppid: int32(num(4)),
		full: true,
		cpu:  float64(num(14)+num(15)) / ticks,
		rss:  num(24) * pageSize,
	}
	if !boot.IsZero() {
		p.started = boot.Add(time.Duration(num(22)) * time.Second / ticks)
	}
	return p, true
}

var boot time.Time

// bootTime is when the machine started, from /proc/stat's btime line, read once.
func bootTime() time.Time {
	if !boot.IsZero() {
		return boot
	}
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "btime "); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); err == nil {
				boot = time.Unix(n, 0)
			}
		}
	}
	return boot
}

// fill is not needed on Linux: list reads everything at once.
func fill(p *proc) { p.full = true }

// Alive reports whether pid names a running process. One owned by another user still counts.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
