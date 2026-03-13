package gitclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"
)

func GenerateUserSSHKeyPair(comment string) (publicKey string, privateKey string, fingerprint string, err error) {
	private, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", "", err
	}

	signerPublicKey, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		return "", "", "", err
	}

	publicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signerPublicKey)))
	if comment = strings.TrimSpace(comment); comment != "" {
		publicKey += " " + comment
	}

	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	}
	privateKey = string(pem.EncodeToMemory(privateKeyBlock))
	fingerprint = ssh.FingerprintSHA256(signerPublicKey)

	return publicKey, privateKey, fingerprint, nil
}
