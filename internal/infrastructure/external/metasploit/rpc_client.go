package metasploit

import (
	"context"
)

type RPCClient struct {
	host     string
	port     int
	user     string
	password string
}

func NewRPCClient(host string, port int, user, password string) *RPCClient {
	return &RPCClient{
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}
}

func (c *RPCClient) ExecuteModule(ctx context.Context, module string, opts map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
