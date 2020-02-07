package daemon

import (
	"os"
	"testing"
)

func TestProcessInfoName(t *testing.T) {
	proc := NewProcessMetaCurrent()
	actual := proc.Name()
	expect := "daemon.test"
	if actual != expect {
		t.Fatal("Name unexpected result", actual, expect)
	}
}

func TestProcessInfoIsCurrentProcess(t *testing.T) {
	proc := NewProcessMetaCurrent()
	actual := proc.IsCurrent()
	expect := true
	if actual != expect {
		t.Fatal("IsCurrent unexpected result", actual, expect)
	}
}

func TestProcessInfoIsDaemon(t *testing.T) {
	proc := NewProcessMetaCurrent()
	actual := proc.IsDaemon()
	expect := false
	if actual != expect {
		t.Fatal("IsDaemon unexpected result", actual, expect)
	}
}

func TestProcessInfoProcess(t *testing.T) {
	proc := NewProcessMetaCurrent()
	actual := proc.Process()
	expect := os.Getpid()
	if actual.Pid != expect {
		t.Fatal("Process unexpected result", actual, expect)
	}
}

func TestProcessInfoParent(t *testing.T) {
	proc := NewProcessMetaCurrent()
	actual := proc.Parent()
	expect := os.Getppid()
	if actual.Pid != expect {
		t.Fatal("Parent unexpected result", actual.Pid, expect)
	}
}
