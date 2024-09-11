package pgxlisten_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/rnovatorov/pgxlisten"
)

type ListenerSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	listener *pgxlisten.Listener
}

func (s *ListenerSuite) SetupSuite() {
	connString := os.Getenv("DATABASE_URL")
	s.Require().NotEmpty(connString)

	pool, err := pgxpool.New(context.Background(), connString)
	s.Require().NoError(err)
	s.Require().NoError(pool.Ping(context.Background()))
	s.pool = pool

	s.listener = pgxlisten.StartListener(s.pool, pgxlisten.WithLogger(log.Default()))
}

func (s *ListenerSuite) TearDownSuite() {
	s.listener.Stop()

	s.pool.Close()
}

func (s *ListenerSuite) TestListenTheSameChannelTwicePanics() {
	c := s.listener.Listen("channel")
	defer c.Unlisten()

	s.Require().Panics(func() {
		s.listener.Listen("channel")
	})
}

func (s *ListenerSuite) TestConnectionResetUponChannelCreation() {
	c := s.listener.Listen("channel")
	defer c.Unlisten()

	notification := s.waitNotification(c)
	s.Require().True(notification.ConnectionReset)
}

func (s *ListenerSuite) TestNoConnectionResetUponNotification() {
	c := s.listener.Listen("channel")
	defer c.Unlisten()
	_ = s.waitNotification(c)

	s.notify("channel", "")

	notification := s.waitNotification(c)
	s.Require().False(notification.ConnectionReset)
}

func (s *ListenerSuite) TestNotificationPayload() {
	c := s.listener.Listen("channel")
	defer c.Unlisten()
	_ = s.waitNotification(c)

	s.notify("channel", "foo")

	notification := s.waitNotification(c)
	s.Require().Equal("foo", notification.Payload)
}

func (s *ListenerSuite) TestTwoChannels() {
	c1 := s.listener.Listen("channel_1")
	defer c1.Unlisten()
	_ = s.waitNotification(c1)

	c2 := s.listener.Listen("channel_2")
	defer c2.Unlisten()
	_ = s.waitNotification(c1)
	_ = s.waitNotification(c2)

	s.notify("channel_1", "foo")
	s.notify("channel_2", "bar")

	notification1 := s.waitNotification(c1)
	s.Require().Equal("foo", notification1.Payload)

	notification2 := s.waitNotification(c2)
	s.Require().Equal("bar", notification2.Payload)
}

func (s *ListenerSuite) notify(name string, payload string) {
	_, err := s.pool.Exec(context.Background(), `
		SELECT pg_notify($1, $2);
	`, name, payload)
	s.Require().NoError(err)
}

func (s *ListenerSuite) waitNotification(c pgxlisten.Channel) pgxlisten.Notification {
	select {
	case notification := <-c.Notifications():
		return notification
	case <-time.After(time.Second):
		s.FailNow("timeout waiting for notification")
		return pgxlisten.Notification{}
	}
}

func TestListener(t *testing.T) {
	suite.Run(t, new(ListenerSuite))
}
