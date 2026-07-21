package nats

import (
	"time"

	"github.com/nats-io/nats.go"
)

type Publisher struct {
	conn *nats.Conn
}

func NewPublisher(conn *nats.Conn) *Publisher {
	return &Publisher{conn: conn}
}

func (p *Publisher) Publish(subject string, data []byte) error {
	return p.conn.Publish(subject, data)
}

func (p *Publisher) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return p.conn.Request(subject, data, timeout)
}
