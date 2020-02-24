package daemon

import (
	"os"
	"strings"
	"testing"
)

func TestProcessInfoName(t *testing.T) {
	proc := NewProcessInfoCurrent()

	actual := proc.FullName
	expect := "godaemon.test"
	if actual != expect {
		t.Fatal("Name unexpected result", actual, expect)
	}

	actual = proc.Name
	expect = "godaemon.test"
	if actual != expect {
		t.Fatal("ShortName unexpected result", actual, expect)
	}

	actual = proc.Path
	expect = "/godaemon.test"
	if strings.HasSuffix(actual, expect) {
		t.Fatal("Path unexpected result", actual, expect)
	}
}

func TestProcessInfoIsCurrentProcess(t *testing.T) {
	proc := NewProcessInfoCurrent()
	actual := proc.IsCurrent()
	expect := true
	if actual != expect {
		t.Fatal("IsCurrent unexpected result", actual, expect)
	}
}

func TestProcessInfoIsDaemon(t *testing.T) {
	proc := NewProcessInfoCurrent()
	actual := proc.IsDaemon()
	expect := false
	if actual != expect {
		t.Fatal("IsDaemon unexpected result", actual, expect)
	}
}

func TestProcessInfoProcess(t *testing.T) {
	proc := NewProcessInfoCurrent()
	actual := proc.Process()
	expect := os.Getpid()
	if actual.Pid != expect {
		t.Fatal("Process unexpected result", actual, expect)
	}
}

func TestProcessInfoParent(t *testing.T) {
	proc := NewProcessInfoCurrent()
	actual := proc.Parent()
	expect := os.Getppid()
	if actual.Pid != expect {
		t.Fatal("Parent unexpected result", actual.Pid, expect)
	}
}
