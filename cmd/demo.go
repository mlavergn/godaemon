package main

import (
	"log"
	"time"

	"github.com/mlavergn/godaemon/src/daemon"
)

func main() {
	log.Println("Go Daemon Demo")

	// only required line
	daemon.NewDaemon().Main()

	log := daemon.Logger()
	log.Println("Go Daemon Demo running ...")

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
