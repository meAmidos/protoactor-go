package main

import (
	"log"
	"runtime"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/remotebenchmark/messages"
	"github.com/AsynkronIT/gam/remoting"
	"github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	runtime.GC()

	remoting.Start("127.0.0.1:8080")
	var sender *actor.PID
	props := actor.
		FromFunc(
			func(context actor.Context) {
				switch msg := context.Message().(type) {
				case *messages.StartRemote:
					log.Println("Starting")
					sender = msg.Sender
					context.Respond(&messages.Start{})
				case *messages.Ping:
					sender.Tell(&messages.Pong{})
				}
			}).
		WithMailbox(actor.NewBoundedMailbox(1000000, 1000000))

	actor.SpawnNamed(props, "remote")

	console.ReadLine()
}
