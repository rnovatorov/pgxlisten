package pgxlisten_test

import (
	"testing"

	"github.com/rnovatorov/pgxlisten"
	"github.com/stretchr/testify/suite"
)

type FanoutSuite struct {
	suite.Suite
	channel channel
	fanout  *pgxlisten.Fanout
}

func (s *FanoutSuite) SetupTest() {
	s.channel = channel{
		notifications: make(chan pgxlisten.Notification),
		done:          make(chan struct{}),
	}
	s.fanout = pgxlisten.StartFanout(s.channel)
}

func (s *FanoutSuite) TearDownTest() {
	s.fanout.Stop()
}

func (s *FanoutSuite) TestOneSub() {
	sub := s.fanout.Listen()
	defer sub.Unlisten()

	s.channel.notifications <- pgxlisten.Notification{Payload: "foo"}

	n := <-sub.Notifications()
	s.Require().Equal("foo", n.Payload)

	select {
	case <-sub.Notifications():
		s.FailNow("unexpected sub notification")
	default:
	}
}

func (s *FanoutSuite) TestTwoSubs() {
	sub1 := s.fanout.Listen()
	defer sub1.Unlisten()

	sub2 := s.fanout.Listen()
	defer sub2.Unlisten()

	s.channel.notifications <- pgxlisten.Notification{Payload: "foo"}

	n1 := <-sub1.Notifications()
	s.Require().Equal("foo", n1.Payload)
	n2 := <-sub2.Notifications()
	s.Require().Equal("foo", n2.Payload)

	select {
	case <-sub1.Notifications():
		s.FailNow("unexpected sub1 notification")
	default:
	}
	select {
	case <-sub2.Notifications():
		s.FailNow("unexpected sub2 notification")
	default:
	}
}

func TestFanout(t *testing.T) {
	suite.Run(t, new(FanoutSuite))
}

type channel struct {
	notifications chan pgxlisten.Notification
	done          chan struct{}
}

func (c channel) Notifications() <-chan pgxlisten.Notification {
	return c.notifications
}

func (c channel) Unlisten() {
	close(c.done)
}
