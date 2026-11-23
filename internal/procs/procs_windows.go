package procs

import (
	"errors"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

// list reads every process and its parent from one Toolhelp snapshot. CPU time, memory and start
// are filled in later, only for processes under a session.
func list() ([]proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []proc
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out = append(out, proc{pid: int32(e.ProcessID), ppid: int32(e.ParentProcessID)})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return out, nil
}

// fill reads one process's start, CPU time and memory.
func fill(p *proc) {
	p.full = true
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.pid))
	if err != nil {
		return
	}
	var created, exited, kernel, user windows.Filetime
	err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user)
	windows.CloseHandle(h)
	if err == nil {
		p.started = time.Unix(0, created.Nanoseconds())
		p.cpu = float64(ticks100ns(kernel)+ticks100ns(user)) / 1e7
	}
	if m, err := (&process.Process{Pid: p.pid}).MemoryInfo(); err == nil {
		p.rss = m.RSS
	}
}

// ticks100ns reads a duration given as a Filetime, in 100-nanosecond units.
func ticks100ns(f windows.Filetime) uint64 {
	return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)
}

// stillActive is the exit code Windows reports for a process that has not exited.
const stillActive = 259

// Alive reports whether pid names a running process. One the app may not open (another user's,
// say) still counts.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return true
	}
	return code == stillActive
}
