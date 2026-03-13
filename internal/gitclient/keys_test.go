package gitclient

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateUserSSHKeyPair(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, fingerprint, err := GenerateUserSSHKeyPair("user@example.com")
	if err != nil {
		t.Fatalf("GenerateUserSSHKeyPair returned error: %v", err)
	}
	if !strings.Contains(publicKey, "user@example.com") {
		t.Fatalf("expected public key comment to include email, got %q", publicKey)
	}
	if !strings.Contains(privateKey, "BEGIN RSA PRIVATE KEY") {
		t.Fatalf("expected PEM private key, got %q", privateKey)
	}
	if !strings.HasPrefix(fingerprint, "SHA256:") {
		t.Fatalf("expected SHA256 fingerprint, got %q", fingerprint)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey)); err != nil {
		t.Fatalf("generated public key is not parseable: %v", err)
	}
	if _, err := ssh.ParseRawPrivateKey([]byte(privateKey)); err != nil {
		t.Fatalf("generated private key is not parseable: %v", err)
	}
}
