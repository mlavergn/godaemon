package main

import (
	oslog "log"
	"time"

	"github.com/mlavergn/godaemon/src/daemon"
)

// standard logger
var log *oslog.Logger

func showProcessInfo() {
	proc := daemon.NewProcessMetaCurrent()
	log.Println("Child name:pid:daemonized", proc.Name(), proc.Pid, proc.IsDaemon())
	parent := proc.Parent()
	log.Println("Parent name:pid", parent.Name(), parent.Pid)
}

func main() {
	oslog.Println("Go Daemon Demo")

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
