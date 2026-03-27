package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
)

func GenerateSshKey() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKey := ssh.MarshalAuthorizedKey(signer.PublicKey())
	return string(privateKeyPEM), string(publicKey), nil
}
