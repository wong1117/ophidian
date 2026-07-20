package network

import (
	"net"
	"net/url"
)

type Proxy struct {
	Type string
	Addr string
	Auth *ProxyAuth
}

type ProxyAuth struct {
	Username string
	Password string
}

func NewProxy(proxyType, addr string) *Proxy {
	return &Proxy{Type: proxyType, Addr: addr}
}
