package procs

import (
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/unix"
)

// list reads every process and its parent with one sysctl call. CPU time and memory are filled
// in later, only for processes under a session.
func list() ([]proc, error) {
	kinfo, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	out := make([]proc, 0, len(kinfo))
	for _, k := range kinfo {
		p := proc{pid: k.Proc.P_pid, ppid: k.Eproc.Ppid}
		tv := k.Proc.P_starttime
		p.started = time.Unix(int64(tv.Sec), int64(tv.Usec)*1000)
		out = append(out, p)
	}
	return out, nil
}

// fill reads one process's CPU time and memory through libproc.
func fill(p *proc) {
	p.full = true
	h := &process.Process{Pid: p.pid}
	if t, err := h.Times(); err == nil {
		p.cpu = t.User + t.System
	}
	if m, err := h.MemoryInfo(); err == nil {
		p.rss = m.RSS
	}
}

// Alive reports whether pid names a running process. One owned by another user still counts.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
