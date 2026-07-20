package secrets

import (
	"testing"
)

func FuzzEncryptDecrypt(f *testing.F) {
	mgr := NewSecretManager(NewMemoryProvider(), "0123456789abcdef0123456789abcdef")

	seeds := []string{"hello", "", "a", "test-password-123", "very long password with special chars: !@#$%^&*()"}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, plaintext string) {
		if len(plaintext) == 0 {
			return
		}
		encrypted, err := mgr.encrypt([]byte(plaintext))
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		decrypted, err := mgr.decrypt(encrypted)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(decrypted) != plaintext {
			t.Errorf("round-trip failed: input=%q output=%q", plaintext, string(decrypted))
		}
	})
}
