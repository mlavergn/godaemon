package daemon

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Daemon type
type Daemon struct {
}

// NewDaemon init
func NewDaemon() *Daemon {
	return &Daemon{}
}

func (id *Daemon) pidFile() *string {
	log.Println("Daemon.pidFile")
	execName, err := os.Executable()
	if err != nil {
		log.Println("Unable to obtain executable name", err)
		return nil
	}
	_, execFile := filepath.Split(execName)
	pidFile := "/tmp/" + execFile + ".lock"
	log.Println(pidFile)
	return &pidFile
}

func (id *Daemon) pidSave(pid int) bool {
	log.Println("Daemon.pidSave")
	lockFile := id.pidFile()
	if lockFile == nil {
		return false
	}

	pidFile, err := os.Create(*lockFile)
	if err != nil {
		log.Println("Unable to create pid file", err)
		return false
	}

	defer pidFile.Close()

	_, err = pidFile.WriteString(strconv.Itoa(pid))
	if err != nil {
		log.Printf("Unable to write pid file", err)
		return false
	}

	pidFile.Sync()
	return true
}

func (id *Daemon) pidRead() int {
	log.Println("Daemon.pidRead")
	lockFile := id.pidFile()
	if lockFile == nil {
		return -1
	}

	_, err := os.Stat(*lockFile)
	if err != nil {
		return -1
	}

	data, err := ioutil.ReadFile(*lockFile)
	if err != nil {
		fmt.Println("Unable to read process lock", err)
		id.pidClear()
		return -1
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		fmt.Println("Unable to parse process id ", err)
		id.pidClear()
		return -1
	}

	return pid
}

func (id *Daemon) pidClear() {
	log.Println("Daemon.pidClear")
	lockFile := id.pidFile()
	os.Remove(*lockFile)
}

// Main export
func (id *Daemon) Main() {
	op := "run"
	if len(os.Args) > 1 {
		op = strings.TrimSpace(strings.ToLower(os.Args[1]))
	}

	switch op {
	case "run":
		fmt.Println("Running ...")
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

		go func() {
			<-signalCh
			signal.Stop(signalCh)
			fmt.Println("Exit command received. Exiting...")
			id.pidClear()
			os.Exit(0)
		}()
		break
	case "start":
		fmt.Println("Starting ...")
		pid := id.pidRead()
		if pid != -1 {
			fmt.Println("Already running or lock file exists")
			os.Exit(1)
		}

		cmd := exec.Command(os.Args[0], "run")
		cmd.Start()
		fmt.Println("Started", cmd.Process.Pid)
		id.pidSave(cmd.Process.Pid)
		os.Exit(0)
	case "stop":
		fmt.Println("Stopping ...")
		pid := id.pidRead()
		if pid == -1 {
			fmt.Println("Not running")
			os.Exit(1)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Println("Unable to find", pid, err)
			os.Exit(1)
		}

		id.pidClear()

		err = process.Kill()
		if err != nil {
			fmt.Println("Failed to stop", pid, err)
		} else {
			fmt.Println("Stopped", pid)
		}
		os.Exit(0)
	case "status":
		pid := id.pidRead()
		if pid != -1 {
			fmt.Println("Process is running or lock file exists")
		} else {
			fmt.Println("Process is stopped")
		}
		os.Exit(0)
	}
}
