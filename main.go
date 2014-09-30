package main

import (
	"strconv"
	"time"

	"github.com/tylertreat/vessel/vessel"
)

func main() {
	vessel := vessel.NewVessel("/_vessel")

	vessel.AddChannel("foo", func(msg string, c chan<- string, done chan<- bool) {
		for x := 0; x < 10; x++ {
			c <- strconv.Itoa(x)
			time.Sleep(time.Second)
		}
		c <- "ping"
		done <- true
	})

	go func() {
		c := time.Tick(5 * time.Second)
		for {
			<-c
			vessel.Publish("baz", "testing 123")
		}
	}()

	vessel.Start(map[string]string{"sockjs": ":8081", "http": ":8082"})
}
