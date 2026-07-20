package secrets

func (m *SecretManager) EncryptForBench(plaintext []byte) ([]byte, error) { return m.encrypt(plaintext) }
func (m *SecretManager) DecryptForBench(encrypted []byte) ([]byte, error) { return m.decrypt(encrypted) }
func NewBenchmarkManager() *SecretManager { return NewSecretManager(NewMemoryProvider(), "0123456789abcdef0123456789abcdef") }
