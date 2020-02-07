package main

import (
	oslog "log"
	"time"

	"github.com/mlavergn/godaemon/src/daemon"
)

// standard logger
var log *oslog.Logger

func showProcessInfo() {
	proc := daemon.NewProcessInfoCurrent()
	oslog.Println("FullName", proc.FullName)
	oslog.Println("Pid:", proc.Pid, proc.IsDaemon())
	oslog.Println("Name:", proc.Name)
	oslog.Println("Path:", proc.Path)
	oslog.Println("IsDaemon:", proc.IsDaemon())
	parent := proc.Parent()
	oslog.Println("Parent name:pid", parent.FullName, parent.Pid)
}

func main() {
	oslog.Println("Go Daemon Demo")

	showProcessInfo()

	log = daemon.Syslogger("demo")
	daemon.Config(true, log)

	// only required line
	daemon.NewDaemon().Main()

	// log = daemon.Logger()
	log.Println("Go Daemon Demo running ...")
	showProcessInfo()

	closeCh := make(chan bool, 1)
	go func() {
		log.Println("Hello")
		<-time.After(10 * time.Second)
		log.Println("Bye")

		// simulate panic
		// panicCh := make(chan bool)
		// close(panicCh)
		// panicCh <- true

		closeCh <- true
	}()

	<-closeCh
}
