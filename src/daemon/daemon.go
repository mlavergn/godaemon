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
const Version = "1.6.0"

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
		} else {
			dlog = oslog.New(ioutil.Discard, "", 0)
		}
		return
	}

	proc := NewProcessInfoCurrent()
	if proc.IsDaemon() {
		log = Syslogger(proc.Name)
		if debug {
			dlog = log
		} else {
			dlog = oslog.New(ioutil.Discard, "", 0)
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

// ProcessInfo export
type ProcessInfo struct {
	Pid       int
	ParentPid int
	Name      string
	FullName  string
	Path      string
}

// NewProcessInfoCurrent current
func NewProcessInfoCurrent() *ProcessInfo {
	return NewProcessInfoPid(os.Getpid())
}

// NewProcessInfoPid export
func NewProcessInfoPid(pid int) *ProcessInfo {
	id := &ProcessInfo{
		Pid:       pid,
		ParentPid: -1,
	}

	// check for parent pid
	if pid != -1 && id.IsCurrent() {
		id.ParentPid = os.Getppid()
	}

	// obtain the executable path
	fullPath := ""
	if pid == os.Getpid() {
		var err error
		fullPath, err = os.Executable()
		if err != nil {
			log.Println("Unable to obtain executable path", err)
			return nil
		}
	} else {
		fullPath = ProcessInfoNameImpl(id.Pid)
	}

	// resolve symlinks
	realPath, symErr := os.Readlink(fullPath)
	if symErr == nil {
		// will fall in here if we resolved a symlink
		fullPath = realPath
	}

	// split the executable path and filename
	basePath, fullName := filepath.Split(fullPath)
	id.FullName = fullName
	if len(basePath) > 0 {
		cwdPath, err := os.Getwd()
		if err == nil && strings.HasPrefix(basePath, cwdPath) {
			basePath = "." + basePath[len(cwdPath):]
		}
		id.Path = basePath
	} else {
		id.Path = "./"
	}

	shortName := fullName
	suffix := strings.LastIndex(shortName, "-")
	if suffix != -1 {
		shortName = shortName[:suffix]
	}
	id.Name = shortName

	return id
}

// IsCurrent export
func (id *ProcessInfo) IsCurrent() bool {
	return id.Pid == os.Getpid()
}

// Parent export
func (id *ProcessInfo) Parent() *ProcessInfo {
	if id.ParentPid != -1 {
		return NewProcessInfoPid(id.ParentPid)
	}
	return nil
}

// FullPath export
func (id *ProcessInfo) FullPath() string {
	return id.Path + id.FullName
}

// IsDaemon determines if the current process qualifies as a daemon
func (id *ProcessInfo) IsDaemon() bool {
	// Indicators of daemon process:
	// 1) We were launched by a startup process (parent PID is a system PID)
	// 2) Current pid is + 1 of parent PID (indicates we were exec'd)
	// 3) The parent PID has the same name as the current executable
	parent := id.Parent()
	return parent.Pid != -1 && (parent.Pid == 1 || id.Pid-parent.Pid == 1 || id.IsForkOf(parent))
}

// Process returns *os.resolveProcess if the process is running, otherwise nil
func (id *ProcessInfo) Process() *os.Process {
	dlog.Println("ProcessInfo.Process")
	process, _ := os.FindProcess(id.Pid)
	err := process.Signal(syscall.Signal(0))
	if err != nil {
		if strings.Contains(err.Error(), "finished") || strings.Contains(err.Error(), "no such process") {
			return nil
		}
		dlog.Println("ProcessInfo.Process signal failed but ignored", err)
	}
	return process
}

// IsForkOf export
func (id *ProcessInfo) IsForkOf(parent *ProcessInfo) bool {
	return id.ParentPid == parent.Pid && id.Name == parent.Name
}

// RestartParent export
func (id *ProcessInfo) RestartParent(updatePath string) {
	dlog.Println("ProcessInfo.RestartParent")
	parent := id.Parent()
	if parent == nil {
		log.Println("ProcessInfo.RestartParent failed to id parent process")
		return
	}
	parentProc := parent.Process()
	dlog.Println("ProcessInfo.RestartParent parent proc", parentProc)
	if parentProc != nil {
		if id.IsForkOf(parent) {
			dlog.Println("ProcessInfo.RestartParent fork confirmed, killing parent", parent.Pid)
			kerr := parentProc.Kill()
			if kerr != nil {
				log.Println("ProcessInfo.RestartParent failed to kill parent", kerr)
				return
			}
		} else {
			log.Println("Fork no detected, attempting restart wihtout kill", id, parent)
		}
		dlog.Println("ProcessInfo.RestartParent killed parent, respawning", updatePath)
		cmd := exec.Command(updatePath, "respawn", "fork")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		serr := cmd.Run()
		if serr != nil {
			log.Println("ProcessInfo.RestartParent failed to respawn parent", serr)
			return
		}
		dlog.Println("ProcessInfo.RestartParent respawned parent", cmd.Process.Pid)
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
		lockPath: pidDir + NewProcessInfoCurrent().Name + ".pid",
	}
	return id
}

func (id *PidFile) set(pid int) bool {
	dlog.Println("PidFile.set")

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
	dlog.Println("PidFile.get")

	_, err := os.Stat(id.lockPath)
	if err != nil {
		dlog.Println("Unable to find process lock", id.lockPath, err)
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
	dlog.Println("PidFile.clear")
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
		process := NewProcessInfoPid(pid).Process()
		if process != nil {
			log.Println("running", pid)
			return true
		}
		// process absent, clear the pid file
		pidFile.clear()
	}
	log.Println("not running")
	return false
}

func (id *Daemon) startHandler(fork bool, lock bool) {
	dlog.Println("Daemon.start fork:lock", fork, lock)

	pidFile := NewPidFile()

	if fork {
		if id.statusHandler() {
			return
		}
		pidFile.clear()

		proc := NewProcessInfoCurrent()
		cmd := exec.Command(proc.FullPath(), "start", "nofork", "lock")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err := cmd.Start()
		if err != nil {
			log.Println("failed to exec", proc.FullPath(), err)
		}
		log.Println("started", cmd.Process.Pid)
		return
	}

	// from this point down, daemon code will be executed

	proc := NewProcessInfoCurrent()
	if lock {
		pidFile.set(proc.Pid)
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		<-signalCh
		// may not appear in log
		log.Println("caught SIGTERM", proc.Pid)
		signal.Stop(signalCh)
		if pidFile.get() == proc.Pid {
			pidFile.clear()
		}
		log.Println("exiting", proc.Pid)
		os.Exit(0)
	}()
}

func (id *Daemon) stopHandler() bool {
	dlog.Println("Daemon.stop")

	pidFile := NewPidFile()
	pid := pidFile.get()
	if pid == -1 {
		log.Println("not running")
		return false
	}

	process := NewProcessInfoPid(pid).Process()
	if process == nil {
		pidFile.clear()
		log.Println("not running")
		return false
	}

	// signal the daemonzied process to terminate and wait
	process.Signal(syscall.SIGTERM)
	process.Wait()

	log.Println("stopped", pid)
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

		proc := NewProcessInfoCurrent()
		cmd := exec.Command(proc.FullPath(), "respawn", "nofork")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err := cmd.Start()
		if err != nil {
			log.Println("failed to exec", proc.FullPath(), err)
		}
		log.Println("started", cmd.Process.Pid)
		return
	}

	// from this point down, daemon code will be executed

	proc := NewProcessInfoCurrent()
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
			proc := NewProcessInfoCurrent()
			log.Println("Respawning", proc.FullPath())
			_, err := os.Lstat(proc.FullPath())
			if err != nil {
				log.Println("Executable not found", proc.FullPath(), err)
				return
			}
			cmd = exec.Command(proc.FullPath(), "start", "nofork", "nolock")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			err = cmd.Run()
			log.Println("Daemon exited", err)
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
