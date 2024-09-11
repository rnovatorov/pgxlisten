package pgxlisten

import "sync"

type Channel interface {
	Notifications() <-chan Notification
	Unlisten()
}

type channel struct {
	notifications chan Notification
	once          sync.Once
	done          chan struct{}
	unlisten      func()
}

func newChannel(unlisten func()) *channel {
	return &channel{
		notifications: make(chan Notification, 1),
		done:          make(chan struct{}),
		unlisten:      unlisten,
	}
}

func (c *channel) Notifications() <-chan Notification {
	return c.notifications
}

func (c *channel) Unlisten() {
	c.once.Do(func() {
		close(c.done)
		c.unlisten()
	})
}
