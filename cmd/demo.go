package main

import (
	"fmt"
	"time"

	"github.com/mlavergn/godaemon/src/daemon"
)

func main() {
	fmt.Println("Go Daemon Demo")

	daemon := daemon.NewDaemon()
	daemon.Main()

	closeCh := make(chan bool, 1)
	go func() {
		fmt.Println("Hello")
		<-time.After(10 * time.Second)

		// simulate panic
		// panicCh := make(chan bool)
		// close(panicCh)
		// panicCh <- true

		closeCh <- true
	}()

	<-closeCh
}
