package procs

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSample(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(self, "-test.run=^$")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Wait()

	s := NewSampler()
	first := s.Sample([]int{os.Getpid(), 1 << 30})
	if first.Self.Procs < 1 || first.Self.RSS == 0 {
		t.Fatalf("self on the first sample: %+v", first.Self)
	}
	if first.Self.HasCPU || first.Machine.HasCPU {
		t.Error("the first sample has nothing to measure CPU against")
	}
	if _, ok := first.ByPID[1<<30]; ok {
		t.Error("a pid that is not running was sampled")
	}
	if first.Machine.MemTotal == 0 || first.Machine.MemUsed == 0 || first.Machine.Cores == 0 {
		t.Errorf("machine: %+v", first.Machine)
	}

	busy := time.Now()
	for time.Since(busy) < 300*time.Millisecond {
	}
	second := s.Sample(nil)
	if !second.Self.HasCPU || second.Self.CPU < 20 {
		t.Errorf("self CPU after a busy wait: %+v", second.Self)
	}
	if !second.Machine.HasCPU {
		t.Error("no machine CPU on the second sample")
	}
	if !Alive(os.Getpid()) || Alive(1<<30) || Alive(0) {
		t.Error("Alive")
	}
	t.Logf("a sample took %s; self %+v", second.Took, second.Self)
}
