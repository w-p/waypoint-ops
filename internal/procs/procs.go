// Package procs samples the machine's load: CPU and memory for the whole machine, and for each
// session's process together with everything under it (the shells, builds and tests it started).
// It reads what the operating system already keeps and starts no processes, so a sample costs a
// few milliseconds. Linux, macOS and Windows each have their own file for listing processes.
package procs

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// Usage is the load of one process and everything under it.
type Usage struct {
	// CPU is percent of one core, as htop shows it, over the time since the last sample. HasCPU is
	// false on the first sample, which has nothing to compare with.
	CPU    float64
	HasCPU bool

	// RSS is resident memory in bytes.
	RSS uint64

	// Procs counts the process and those under it.
	Procs int

	// Started is when the process itself started.
	Started time.Time
}

// Machine is the load of the whole machine.
type Machine struct {
	// CPU is percent of all cores in use since the last sample; HasCPU as for Usage.
	CPU    float64
	HasCPU bool

	MemUsed  uint64
	MemTotal uint64
	Cores    int
}

// Reading is one sample.
type Reading struct {
	At      time.Time
	Machine Machine

	// ByPID holds the usage of each process asked about that is running.
	ByPID map[int]Usage

	// Self is this app's own usage.
	Self Usage

	// Took is how long the sample took to read.
	Took time.Duration
}

// proc is one process as listed: its parent, and when full, its CPU time, memory and start.
type proc struct {
	pid, ppid int32
	full      bool
	cpu       float64 // seconds, user and system
	rss       uint64
	started   time.Time
}

// seen is what the sampler remembers of a process between samples.
type seen struct {
	cpu     float64
	started time.Time
}

// Sampler takes samples and remembers the last, so CPU can be measured as use over time.
type Sampler struct {
	prevAt   time.Time
	prev     map[int32]seen
	prevBusy float64
	prevAll  float64
	cores    int
}

// NewSampler makes a sampler. Its first sample has no CPU figures.
func NewSampler() *Sampler {
	cores, _ := cpu.Counts(true)
	return &Sampler{prev: map[int32]seen{}, cores: cores}
}

// Sample reads the machine and the process trees under roots. Roots that are not running are
// left out of ByPID.
func (s *Sampler) Sample(roots []int) Reading {
	begin := time.Now()
	now := begin
	out := Reading{At: now, ByPID: map[int]Usage{}}
	s.machine(&out.Machine)

	list, err := list()
	if err != nil {
		s.prevAt = now
		out.Took = time.Since(begin)
		return out
	}
	byPID := make(map[int32]*proc, len(list))
	children := map[int32][]int32{}
	for i := range list {
		p := &list[i]
		byPID[p.pid] = p
		if p.ppid != p.pid {
			children[p.ppid] = append(children[p.ppid], p.pid)
		}
	}

	self := os.Getpid()
	wanted := append([]int{self}, roots...)
	next := map[int32]seen{}
	elapsed := now.Sub(s.prevAt).Seconds()
	for _, root := range wanted {
		if _, done := out.ByPID[root]; done || root <= 0 {
			continue
		}
		top, ok := byPID[int32(root)]
		if !ok {
			continue
		}
		var u Usage
		var used float64
		stack := []int32{top.pid}
		visited := map[int32]bool{}
		for len(stack) > 0 {
			pid := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[pid] {
				continue
			}
			visited[pid] = true
			p := byPID[pid]
			if p == nil {
				continue
			}
			if !p.full {
				fill(p)
			}
			u.Procs++
			u.RSS += p.rss
			if p.pid == top.pid {
				u.Started = p.started
			}
			next[pid] = seen{p.cpu, p.started}
			if before, ok := s.prev[pid]; ok && before.started.Equal(p.started) {
				used += max(0, p.cpu-before.cpu)
			} else if !s.prevAt.IsZero() && p.started.After(s.prevAt) {
				used += p.cpu // started since the last sample: all its time is new
			}
			stack = append(stack, children[pid]...)
		}
		if !s.prevAt.IsZero() && elapsed > 0 {
			u.CPU = 100 * used / elapsed
			u.HasCPU = true
		}
		if root == self {
			out.Self = u
		} else {
			out.ByPID[root] = u
		}
	}
	s.prev = next
	s.prevAt = now
	out.Took = time.Since(begin)
	return out
}

// machine reads the machine's CPU and memory.
func (s *Sampler) machine(m *Machine) {
	m.Cores = s.cores
	if v, err := mem.VirtualMemory(); err == nil {
		m.MemUsed, m.MemTotal = v.Used, v.Total
	}
	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		return
	}
	t := times[0]
	all := t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal
	busy := all - t.Idle - t.Iowait
	if s.prevAll > 0 && all > s.prevAll {
		m.CPU = 100 * max(0, busy-s.prevBusy) / (all - s.prevAll)
		m.HasCPU = true
	}
	s.prevBusy, s.prevAll = busy, all
}
