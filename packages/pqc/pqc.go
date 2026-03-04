package pqc

import (
	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/dilithium/mode3"
)

var scheme sign.Scheme = mode3.Scheme()

// GenerateKeypair creates a new Dilithium3 keypair. Returns raw public and private key bytes.
func GenerateKeypair() (publicKey, privateKey []byte, err error) {
	pk, sk, err := scheme.GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	pubBytes, err := pk.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	privBytes, err := sk.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	return pubBytes, privBytes, nil
}

// Sign signs message with the given private key (raw bytes from GenerateKeypair). Returns signature bytes.
func Sign(privateKey, message []byte) (signature []byte) {
	sk, err := scheme.UnmarshalBinaryPrivateKey(privateKey)
	if err != nil {
		return nil
	}
	sig := scheme.Sign(sk, message, nil)
	return sig
}

// Verify returns true if signature is a valid signature of message by the given public key.
func Verify(publicKey, message, signature []byte) bool {
	pk, err := scheme.UnmarshalBinaryPublicKey(publicKey)
	if err != nil {
		return false
	}
	return scheme.Verify(pk, message, signature, nil)
}
