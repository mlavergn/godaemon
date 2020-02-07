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

// ----------------------------------------------------------------------------
// init

// Version export
const Version = "1.5.0"

// DEBUG flag
const DEBUG = false

// standard logger
var log *oslog.Logger

// debug logger
var dlog *oslog.Logger

// Logger returns configred *os.Logger
func Logger() *oslog.Logger {
	return log
}

// Syslogger returns a configred *os.Logger logging to syslogd
func Syslogger(procname string) *oslog.Logger {
	syslogWriter, err := syslog.New(syslog.LOG_WARNING, procname)
	if err == nil {
		return oslog.New(syslogWriter, "", 0)
	}
	return oslog.New(os.Stderr, "", 0)
}

// Config export
func Config(debug bool, logger *oslog.Logger) {
	if logger != nil {
		log = logger
		if debug {
			dlog = logger
		}
		return
	}

	proc := NewProcessMetaCurrent()
	if proc.IsDaemon() {
		log = Syslogger(proc.Name())
		if debug {
			dlog = log
		}
		return
	}

	log = oslog.New(os.Stderr, "", 0)
	if debug {
		dlog = oslog.New(os.Stdout, "GoDaemon ", oslog.Ltime|oslog.Lshortfile)
	} else {
		dlog = oslog.New(ioutil.Discard, "", 0)
	}
}

func init() {
	Config(DEBUG, nil)
}

// ----------------------------------------------------------------------------
// process

// ProcessMeta export
type ProcessMeta struct {
	Pid       int
	ParentPid int
}

// NewProcessMetaCurrent current
func NewProcessMetaCurrent() *ProcessMeta {
	return NewProcessMetaPid(os.Getpid())
}

// NewProcessMetaPid export
func NewProcessMetaPid(pid int) *ProcessMeta {
	id := &ProcessMeta{
		Pid:       pid,
		ParentPid: -1,
	}
	if pid != -1 && id.IsCurrent() {
		id.ParentPid = os.Getppid()
	}
	return id
}

// IsCurrent export
func (id *ProcessMeta) IsCurrent() bool {
	return id.Pid == os.Getpid()
}

// Parent export
func (id *ProcessMeta) Parent() *ProcessMeta {
	if id.ParentPid != -1 {
		return NewProcessMetaPid(id.ParentPid)
	}
	return nil
}

// Name export
func (id *ProcessMeta) Name() string {
	if id.IsCurrent() {
		execPath, err := os.Executable()
		if err != nil {
			log.Println("Unable to obtain executable path", err)
			return ""
		}
		_, execName := filepath.Split(execPath)
		return execName
	}
	return processMetaNameImpl(id.Pid)
}

// IsDaemon determines if the current process qualifies as a daemon
func (id *ProcessMeta) IsDaemon() bool {
	// Indicators of daemon process:
	// 1) We were launched by a startup process (parent PID is a system PID)
	// 2) Current pid is + 1 of parent PID (indicates we were exec'd)
	// 3) The parent PID has the same name as the current executable
	parent := id.Parent()
	return parent.Pid != -1 && (parent.Pid == 1 || id.Pid-parent.Pid == 1 || id.IsForkOf(parent))
}

// SemanticlessName name stripped of any version suffix
func (id *ProcessMeta) SemanticlessName() string {
	// strip version suffix eg. service-1.0.0 -> service
	name := id.Name()
	suffix := strings.LastIndex(name, "-")
	if suffix != -1 {
		name = name[:suffix]
	}

	return name
}

// Process returns *os.resolveProcess if the process is running, otherwise nil
func (id *ProcessMeta) Process() *os.Process {
	process, _ := os.FindProcess(id.Pid)
	err := process.Signal(syscall.Signal(0))
	if err != nil {
		if strings.Contains(err.Error(), "finished") || strings.Contains(err.Error(), "no such process") {
			return nil
		}
		log.Println(err)
	}
	return process
}

// IsForkOf export
func (id *ProcessMeta) IsForkOf(parent *ProcessMeta) bool {
	return id.ParentPid == parent.Pid && id.Name() == parent.Name()
}

// RestartParent export
func (id *ProcessMeta) RestartParent() {
	parent := id.Parent()
	if parent == nil {
		return
	}
	proc := parent.Process()
	if proc != nil {
		if id.IsForkOf(parent) {
			proc.Kill()
			cmd := exec.Command(os.Args[0], "respawn")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			cmd.Start()
			os.Exit(0)
		}
	}
}

// ----------------------------------------------------------------------------
// pid

// PidFile type
type PidFile struct {
	lockPath string
}

// NewPidFile init
func NewPidFile() *PidFile {
	// unix convention eg. /var/run/service.pid
	pidDir := "/var/run/"
	var stat syscall.Stat_t
	if syscall.Stat(pidDir, &stat) == nil {
		dirUID := int(stat.Uid)
		if dirUID != os.Geteuid() {
			// can't write to /var/run, use /tmp
			pidDir = "/tmp/"
		}
	}

	id := &PidFile{
		lockPath: pidDir + NewProcessMetaCurrent().Name() + ".pid",
	}
	return id
}

func (id *PidFile) set(pid int) bool {
	log.Println("PidFile.set")

	pidFile, err := os.Create(id.lockPath)
	if err != nil {
		log.Println("Unable to create pid file", id.lockPath, err)
		return false
	}

	defer pidFile.Close()

	_, err = pidFile.WriteString(strconv.Itoa(pid))
	if err != nil {
		log.Println("Unable to write pid file", id.lockPath, err)
		return false
	}

	pidFile.Sync()
	return true
}

func (id *PidFile) get() int {
	log.Println("PidFile.get")

	_, err := os.Stat(id.lockPath)
	if err != nil {
		log.Println("Unable to find process lock", id.lockPath, err)
		return -1
	}

	data, err := ioutil.ReadFile(id.lockPath)
	if err != nil {
		log.Println("Unable to read process lock", id.lockPath, err)
		id.clear()
		return -1
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		log.Println("Unable to parse process id ", id.lockPath, err)
		id.clear()
		return -1
	}

	return pid
}

func (id *PidFile) clear() {
	log.Println("PidFile.clear")
	os.Remove(id.lockPath)
}

// ----------------------------------------------------------------------------
// Daemon

// Daemon type
type Daemon struct {
}

// NewDaemon init
func NewDaemon() *Daemon {
	return &Daemon{}
}

// initd handlers

func (id *Daemon) statusHandler() bool {
	dlog.Println("Daemon.status")
	pidFile := NewPidFile()
	pid := pidFile.get()
	if pid != -1 {
		process := NewProcessMetaPid(pid).Process()
		if process != nil {
			log.Println("Process is running", pid)
			return true
		}
		// process absent, clear the pid file
		pidFile.clear()
	}
	log.Println("Process not running")
	return false
}

func (id *Daemon) startHandler(fork bool, lock bool) {
	log.Println("Daemon.start fork:lock", fork, lock)

	pidFile := NewPidFile()

	if fork {
		if id.statusHandler() {
			return
		}
		pidFile.clear()

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

	proc := NewProcessMetaCurrent()
	if lock {
		pidFile.set(proc.Pid)
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		<-signalCh
		// may not appear in log
		log.Println("Exiting", proc.Pid)
		signal.Stop(signalCh)
		if pidFile.get() == proc.Pid {
			pidFile.clear()
		}
		os.Exit(0)
	}()
}

func (id *Daemon) stopHandler() bool {
	dlog.Println("Daemon.stop")

	pidFile := NewPidFile()
	pid := pidFile.get()
	if pid == -1 {
		log.Println("Not running")
		return false
	}

	process := NewProcessMetaPid(pid).Process()
	if process == nil {
		pidFile.clear()
		log.Println("Not running, cleared pid")
		return false
	}

	// signal the daemonzied process to terminate and wait
	process.Signal(syscall.SIGTERM)
	process.Wait()

	log.Println("Stopped", pid)
	pidFile.clear()
	return true
}

// Mimics an inittab respawnHandler entry
func (id *Daemon) respawnHandler(fork bool) {
	dlog.Println("Daemon.respawn fork", fork)

	pidFile := NewPidFile()

	if fork {
		if id.statusHandler() {
			return
		}
		pidFile.clear()

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

	proc := NewProcessMetaCurrent()
	pidFile.set(proc.Pid)

	exitCh := make(chan bool, 1)
	defer close(exitCh)

	var cmd *exec.Cmd

	go func() {
		signalCh := make(chan os.Signal, 1)
		// signal.Notify(signalCh, os.Kill, syscall.SIGTERM)
		signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)
		defer close(signalCh)

		<-signalCh
		log.Println("Stopping respawn", proc.Pid)
		signal.Stop(signalCh)
		// clear the pid lock
		if proc.Pid == pidFile.get() {
			pidFile.clear()
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
			log.Println("Respawn stopped", proc.Pid)
			return
		default:
			log.Println("Respawning", os.Args[0])
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
		// if we we not passed a fork parameter, default to fork  and lock true
		if len(os.Args) == 2 {
			fork = true
			lock = true
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
