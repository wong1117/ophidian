package nats

import (
	"github.com/nats-io/nats.go"
)

type Config struct {
	URL      string
	Username string
	Password string
}

func NewConnection(cfg Config) (*nats.Conn, error) {
	return nats.Connect(cfg.URL, nats.UserInfo(cfg.Username, cfg.Password))
}
