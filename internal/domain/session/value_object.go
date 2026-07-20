package session

type Protocol string

const (
	ProtocolTCP  Protocol = "TCP"
	ProtocolHTTP Protocol = "HTTP"
	ProtocolHTTPS Protocol = "HTTPS"
	ProtocolDNS  Protocol = "DNS"
	ProtocolICMP Protocol = "ICMP"
	ProtocolSMB  Protocol = "SMB"
)

type Encryption string

const (
	EncryptionNone       Encryption = "NONE"
	EncryptionTLS        Encryption = "TLS"
	EncryptionChaCha20  Encryption = "CHACHA20_POLY1305"
	EncryptionAES       Encryption = "AES_256_GCM"
)
