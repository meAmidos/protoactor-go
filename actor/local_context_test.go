package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLocalContext_SpawnNamed(t *testing.T) {
	pid, p := spawnMockProcess("foo/bar")
	defer removeMockProcess(pid)
	p.On("SendSystemMessage", matchPID(pid), mock.Anything)

	props := &Props{
		spawner: func(id string, _ *Props, _ *PID) (*PID, error) {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id), nil
		},
	}

	parent := &localContext{self: NewLocalPID("foo")}
	parent.SpawnNamed(props, "bar")
	mock.AssertExpectationsForObjects(t, p)
}

// TestLocalContext_Stop verifies if context is stopping and receives a Watch message, it should
// immediately respond with a Terminated message
func TestLocalContext_Stop(t *testing.T) {
	pid, p := spawnMockProcess("foo")
	defer removeMockProcess(pid)

	other, o := spawnMockProcess("watcher")
	defer removeMockProcess(other)

	o.On("SendSystemMessage", other, &Terminated{Who: pid})

	lc := newLocalContext(nullProducer, DefaultSupervisorStrategy(), nil, nil, nil)
	lc.self = pid
	lc.InvokeSystemMessage(&Stop{})
	lc.InvokeSystemMessage(&Watch{Watcher: other})

	mock.AssertExpectationsForObjects(t, p, o)
}

func TestLocalContext_SendMessage_WithOutboundMiddleware(t *testing.T) {
	// Define a local context with no-op outbound middlware
	mw := func(next SenderFunc) SenderFunc {
		return func(ctx Context, target *PID, envelope MessageEnvelope) {
			next(ctx, target, envelope)
		}
	}

	ctx := &localContext{
		actor: nullReceive,
		outboundMiddleware: makeOutboundMiddlewareChain(
			[]func(SenderFunc) SenderFunc{mw}, localContextSender,
		),
	}

	ctx.SetBehavior(nullReceive.Receive)

	// Define a receiver to which the local context will send a message
	var counter int
	receiver := Spawn(FromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case bool:
			counter++
		}
	}))

	// Send a message with Tell
	// Then wait a little to allow the receiver to process the message
	// TODO: There should be a better way to wait.
	timeout := 3 * time.Millisecond
	ctx.Tell(receiver, true)
	time.Sleep(timeout)
	assert.Equal(t, 1, counter)

	// Send a message with Request
	counter = 0 // Reset the counter
	ctx.Request(receiver, true)
	time.Sleep(3 * time.Millisecond)
	assert.Equal(t, 1, counter)

	// Send a message with RequestFuture
	counter = 0 // Reset the counter
	ctx.RequestFuture(receiver, true, timeout).Wait()
	assert.Equal(t, 1, counter)
}

func BenchmarkLocalContext_ProcessMessageNoMiddleware(b *testing.B) {
	var m interface{} = 1

	ctx := &localContext{actor: nullReceive}
	ctx.SetBehavior(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func BenchmarkLocalContext_ProcessMessageWithMiddleware(b *testing.B) {
	var m interface{} = 1

	fn := func(next ActorFunc) ActorFunc {
		fn := func(context Context) {
			next(context)
		}
		return fn
	}

	ctx := &localContext{actor: nullReceive, middleware: makeMiddlewareChain([]func(ActorFunc) ActorFunc{fn, fn}, localContextReceiver)}
	ctx.SetBehavior(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func TestActorContinueFutureInActor(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	pid := Spawn(FromFunc(func(ctx Context) {
		if ctx.Message() == "request" {
			ctx.Respond("done")
		}
		if ctx.Message() == "start" {
			f := ctx.RequestFuture(ctx.Self(), "request", 5*time.Second)
			ctx.AwaitFuture(f, func(res interface{}, err error) {
				wg.Done()
			})
		}
	}))
	pid.Tell("start")
	wg.Wait()
}
