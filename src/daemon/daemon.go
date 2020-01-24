package daemon

import (
	"io/ioutil"
	oslog "log"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Version export
const Version = "1.3.2"

// standard logger
var log *oslog.Logger

// debug logger
var dlog *oslog.Logger

// IsDaemon determines if proc can be assumed to be a daemon
func IsDaemon() bool {
	// if we were launched by a startup process -or- our group is a system
	return os.Getppid() < 100 || os.Getpid()-os.Getppid() == 1
}

// ServiceName generates a daemon name from the current executable
func ServiceName() string {
	execName, err := os.Executable()
	if err != nil {
		log.Println("Unable to obtain executable name", err)
		return "unknown"
	}
	_, svcName := filepath.Split(execName)

	// strip version suffix eg. service-1.0.0 -> service
	verSuffix := strings.LastIndex(svcName, "-")
	if verSuffix != -1 {
		svcName = svcName[:verSuffix]
	}

	return svcName
}

// ServiceProcess returns *os.Process if the service is running, otherwise nil
func ServiceProcess(pid int) *os.Process {
	process, _ := os.FindProcess(pid)
	err := process.Signal(syscall.Signal(0))
	if err != nil {
		if strings.Contains(err.Error(), "finished") || strings.Contains(err.Error(), "no such process") {
			return nil
		}
		log.Println(err)
	}
	return process
}

// ServiceLogger returns configred *os.Logger
func ServiceLogger() *oslog.Logger {
	return log
}

func init() {
	if IsDaemon() {
		logSys, err := syslog.New(syslog.LOG_WARNING, ServiceName())
		if err == nil {
			log = oslog.New(logSys, "", 0)
			log.Println("syslog logging enabled")
		} else {
			log = oslog.New(ioutil.Discard, "", 0)
		}
	} else {
		log = oslog.New(os.Stdout, "", 0)
	}

	// for debugging uncomment:
	// dlog = oslog.New(ioutil.Discard, "GoDaemon", oslog.Ltime|oslog.Lshortfile)
	dlog = oslog.New(ioutil.Discard, "", 0)
}

// Daemon type
type Daemon struct {
}

// NewDaemon init
func NewDaemon() *Daemon {
	return &Daemon{}
}

func (id *Daemon) pidFile() *string {
	dlog.Println("Daemon.pidFile")
	pidDir := "/var/run/"
	var stat syscall.Stat_t
	if syscall.Stat(pidDir, &stat) == nil {
		dirUID := int(stat.Uid)
		if dirUID != os.Geteuid() && dirUID != os.Geteuid() {
			// can't write to /var/run, use /tmp
			pidDir = "/tmp/"
		}
	}

	// unix convention eg. /var/run/service.pid
	pidFile := pidDir + ServiceName() + ".pid"
	dlog.Println(pidFile)
	return &pidFile
}

func (id *Daemon) pidSave(pid int) bool {
	dlog.Println("Daemon.pidSave")
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
		log.Println("Unable to write pid file", err)
		return false
	}

	pidFile.Sync()
	return true
}

func (id *Daemon) pidRead() int {
	dlog.Println("Daemon.pidRead")
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
		log.Println("Unable to read process lock", err)
		id.pidClear()
		return -1
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		log.Println("Unable to parse process id ", err)
		id.pidClear()
		return -1
	}

	return pid
}

func (id *Daemon) pidClear() {
	dlog.Println("Daemon.pidClear")
	lockFile := id.pidFile()
	os.Remove(*lockFile)
}

func (id *Daemon) status() {
	dlog.Println("Daemon.status")
	pid := id.pidRead()
	if pid != -1 {
		process := ServiceProcess(pid)
		if process != nil {
			log.Println("Process is running", pid)
			os.Exit(1)
		}
		// process absent, clear the pid file
		id.pidClear()
	}
	log.Println("Process is stopped")
}

func (id *Daemon) start() {
	dlog.Println("Daemon.start")
	pid := id.pidRead()
	if pid != -1 {
		process := ServiceProcess(pid)
		if process != nil {
			log.Println("Process already running", pid)
			os.Exit(1)
		}
		// process absent, clear the pid file
		id.pidClear()
	}

	cmd := exec.Command(os.Args[0])
	cmd.Start()
	log.Println("Started", cmd.Process.Pid)
	id.pidSave(cmd.Process.Pid)
}

func (id *Daemon) stop() {
	dlog.Println("Daemon.stop")
	pid := id.pidRead()
	if pid == -1 {
		log.Println("Not running")
		os.Exit(1)
	}

	process := ServiceProcess(pid)
	if process == nil {
		id.pidClear()
		log.Println("Not running, cleared pid")
		os.Exit(1)
	}

	err := process.Kill()
	if err != nil {
		log.Println("Failed to stop", pid, err)
		os.Exit(1)
	}

	log.Println("Stopped", pid)
	id.pidClear()
}

func (id *Daemon) run() {
	dlog.Println("Daemon.run")
	log.Println("process run command")
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		<-signalCh
		// may not appear in log
		log.Println("process stop command")
		signal.Stop(signalCh)
		id.pidClear()
		os.Exit(0)
	}()
}

// Main export
func (id *Daemon) Main() {
	dlog.Println("Daemon.Main")
	op := "run"
	if len(os.Args) > 1 {
		op = strings.TrimSpace(strings.ToLower(os.Args[1]))
	}

	switch op {
	case "run":
		dlog.Println("Running ...")
		id.run()
		break
	case "start":
		dlog.Println("Starting ...")
		id.start()
		os.Exit(0)
	case "stop":
		dlog.Println("Stopping ...")
		id.stop()
		os.Exit(0)
	case "restart":
		dlog.Println("Restarting ...")
		id.stop()
		id.start()
		os.Exit(0)
	case "status":
		dlog.Println("Status ...")
		id.status()
		os.Exit(0)
	}
}
