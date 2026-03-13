package gitclient

import "testing"

func TestEncryptSecretRoundTrip(t *testing.T) {
	t.Parallel()

	secret := []byte("top-secret")
	encrypted, err := EncryptSecret(secret, "ssh-private-key")
	if err != nil {
		t.Fatalf("EncryptSecret returned error: %v", err)
	}
	if encrypted == "" {
		t.Fatalf("expected encrypted payload")
	}

	decrypted, err := DecryptSecret(secret, encrypted)
	if err != nil {
		t.Fatalf("DecryptSecret returned error: %v", err)
	}
	if decrypted != "ssh-private-key" {
		t.Fatalf("DecryptSecret=%q want %q", decrypted, "ssh-private-key")
	}
}
