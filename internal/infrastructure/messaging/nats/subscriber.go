package nats

import (
	"github.com/nats-io/nats.go"
)

type Subscriber struct {
	conn *nats.Conn
}

func NewSubscriber(conn *nats.Conn) *Subscriber {
	return &Subscriber{conn: conn}
}

func (s *Subscriber) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return s.conn.Subscribe(subject, handler)
}

func (s *Subscriber) QueueSubscribe(subject, queue string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return s.conn.QueueSubscribe(subject, queue, handler)
}
