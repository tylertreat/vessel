package main

import (
	"time"

	"github.com/tylertreat/vessel/vessel"
)

func main() {
	vessel := vessel.NewSockJSVessel("/vessel")

	vessel.AddChannel("foo", func(msg string, c chan<- string, done chan<- bool) {
		c <- "ping"
		done <- true
	})

	go func() {
		c := time.Tick(5 * time.Second)
		for {
			<-c
			vessel.Broadcast("baz", "testing 123")
		}
	}()

	vessel.Start(":8081", ":8082")
}
