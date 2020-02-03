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
const Version = "1.4.0"

// standard logger
var log *oslog.Logger

// debug logger
var dlog *oslog.Logger

// Package level functions

// IsDaemon determines if the current process qualifies as a daemon
func IsDaemon() bool {
	// Indicators of daemon process:
	// 1) We were launched by a startup process (parent PID is a system PID)
	// 2) Current pid is + 1 of parent PID (indicates we were exec'd)
	// 3) An arg count of 3 or more indicates we have a fork flag
	return os.Getppid() < 100 || os.Getpid()-os.Getppid() == 1 || len(os.Args) >= 3
}

// Name generates a daemon name from the current executable
// using the executable stripped of version suffixes
func Name() string {
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

// resolveProcess returns *os.resolveProcess if the service is running, otherwise nil
func resolveProcess(pid int) *os.Process {
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

// Logger returns configred *os.Logger
func Logger() *oslog.Logger {
	return log
}

func init() {
	if IsDaemon() {
		syslogWriter, err := syslog.New(syslog.LOG_WARNING, Name())
		if err == nil {
			log = oslog.New(syslogWriter, "", 0)
		} else {
			log = oslog.New(ioutil.Discard, "", 0)
		}
		dlog = oslog.New(ioutil.Discard, "", 0)
		// for debugging uncomment:
		// dlog = oslog.New(syslogWriter, "GoDaemon ", oslog.Ltime|oslog.Lshortfile)
	} else {
		log = oslog.New(os.Stderr, "", 0)
		dlog = oslog.New(ioutil.Discard, "", 0)
		// for debugging uncomment:
		// 	dlog = oslog.New(os.Stdout, "GoDaemon ", oslog.Ltime|oslog.Lshortfile)
	}
}

// Daemon type
type Daemon struct {
}

// NewDaemon init
func NewDaemon() *Daemon {
	return &Daemon{}
}

// PID lock file methods

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
	pidFile := pidDir + Name() + ".pid"
	return &pidFile
}

func (id *Daemon) pidSet(pid int) bool {
	dlog.Println("Daemon.pidSet")
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

func (id *Daemon) pidGet() int {
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

// initd handlers

func (id *Daemon) statusHandler() bool {
	dlog.Println("Daemon.status")
	pid := id.pidGet()
	if pid != -1 {
		process := resolveProcess(pid)
		if process != nil {
			log.Println("Process is running", pid)
			return true
		}
		// process absent, clear the pid file
		id.pidClear()
	}
	log.Println("Process not running")
	return false
}

func (id *Daemon) startHandler(fork bool, lock bool) {
	dlog.Println("Daemon.start fork", fork)

	if fork {
		if id.statusHandler() {
			return
		}
		id.pidClear()

		cmd := exec.Command(os.Args[0], "start", "nofork", "lock")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err := cmd.Start()
		if err != nil {
			log.Println(err)
		}
		log.Println("Started", cmd.Process.Pid)
		return
	}

	// from this point down, daemon code will be executed

	if lock {
		id.pidSet(os.Getpid())
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		<-signalCh
		// may not appear in log
		log.Println("Exiting", os.Getpid())
		signal.Stop(signalCh)
		if os.Getpid() == id.pidGet() {
			id.pidClear()
		}
		os.Exit(0)
	}()
}

func (id *Daemon) stopHandler() bool {
	dlog.Println("Daemon.stop")
	pid := id.pidGet()
	if pid == -1 {
		log.Println("Not running")
		return false
	}

	process := resolveProcess(pid)
	if process == nil {
		id.pidClear()
		log.Println("Not running, cleared pid")
		return false
	}

	// signal the daemonzied process to terminate and wait
	process.Signal(syscall.SIGTERM)
	process.Wait()

	log.Println("Stopped", pid)
	id.pidClear()
	return true
}

// Mimics an inittab respawnHandler entry
func (id *Daemon) respawnHandler(fork bool) {
	dlog.Println("Daemon.respawn fork", fork)

	if fork {
		if id.statusHandler() {
			return
		}
		id.pidClear()

		cmd := exec.Command(os.Args[0], "respawn", "nofork")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err := cmd.Start()
		if err != nil {
			log.Println(err)
		}
		log.Println("Started", cmd.Process.Pid)
		return
	}

	// from this point down, daemon code will be executed

	id.pidSet(os.Getpid())

	exitCh := make(chan bool, 1)
	defer close(exitCh)

	var cmd *exec.Cmd

	go func() {
		signalCh := make(chan os.Signal, 1)
		// signal.Notify(signalCh, os.Kill, syscall.SIGTERM)
		signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)
		defer close(signalCh)

		<-signalCh
		log.Println("Stopping respawn", os.Getpid())
		signal.Stop(signalCh)
		// clear the pid lock
		if os.Getpid() == id.pidGet() {
			id.pidClear()
		}
		// kill the managed process
		if cmd != nil {
			cmd.Process.Signal(syscall.SIGTERM)
		}
		// signal it's safe to exit
		exitCh <- true
	}()

	for {
		select {
		case <-exitCh:
			log.Println("Respawn stopped", os.Getpid())
			return
		default:
			log.Println("Respawning ...")
			cmd = exec.Command(os.Args[0], "start", "nofork", "nolock")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			cmd.Run()
			log.Println("Daemon exited")
		}
	}
}

// Main export
func (id *Daemon) Main() {
	dlog.Println("Daemon.Main")
	op := "start"
	fork := false
	lock := false
	if len(os.Args) > 1 {
		op = strings.TrimSpace(strings.ToLower(os.Args[1]))
		// if we we not passed a fork parameter, defaul to fork true
		if len(os.Args) == 2 {
			fork = true
		}
	}
	if len(os.Args) > 2 {
		fork = strings.TrimSpace(strings.ToLower(os.Args[2])) == "fork"
	}
	if len(os.Args) > 3 {
		lock = strings.TrimSpace(strings.ToLower(os.Args[3])) == "lock"
	}

	switch op {
	case "start":
		dlog.Println("Starting ...")
		id.startHandler(fork, lock)
		if fork {
			// only exit if forked
			os.Exit(0)
		}
	case "stop":
		dlog.Println("Stopping ...")
		id.stopHandler()
		os.Exit(0)
	case "restart":
		dlog.Println("Restarting ...")
		id.stopHandler()
		id.startHandler(true, true)
		os.Exit(0)
	case "status":
		dlog.Println("Status ...")
		id.statusHandler()
		os.Exit(0)
	case "respawn":
		dlog.Println("Respawn ...")
		id.respawnHandler(fork)
		os.Exit(0)
	}
}
