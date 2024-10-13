package pgxlisten

import (
	"sync"
)

type Fanout struct {
	in       Channel
	ids      uint64
	stopping chan struct{}
	stopped  chan struct{}
	mu       sync.Mutex
	out      map[uint64]*channel
}

func StartFanout(c Channel) *Fanout {
	f := &Fanout{
		in:       c,
		stopping: make(chan struct{}),
		stopped:  make(chan struct{}),
		out:      make(map[uint64]*channel),
	}

	go func() {
		defer close(f.stopped)
		f.run()
	}()

	return f
}

func (f *Fanout) Stop() {
	close(f.stopping)
	<-f.stopped
}

func (f *Fanout) Listen() Channel {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := f.nextID()

	unlisten := func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		delete(f.out, id)
	}

	s := newChannel(unlisten)
	f.out[id] = s

	return s
}

func (f *Fanout) run() {
	for {
		select {
		case <-f.stopping:
			return
		case notification := <-f.in.Notifications():
			f.fanout(notification)
		}
	}
}

func (f *Fanout) fanout(n Notification) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.out {
		select {
		case <-c.done:
		case c.notifications <- n:
		}
	}
}

func (f *Fanout) nextID() uint64 {
	f.ids++
	return f.ids
}
