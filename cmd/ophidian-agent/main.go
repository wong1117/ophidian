package main

import (
	"log"
	"time"
)

type AgentConfig struct {
	ServerURL         string
	HeartbeatInterval time.Duration
	EncryptionKey     string
}

func main() {
	// TODO: implement agent runtime
	log.Println("ophidian-agent starting...")
}
