package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

type Signer struct {
	privateKey *ecdsa.PrivateKey
	address    string
}

func NewSigner(privateKeyHex string) (*Signer, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, err
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &Signer{
		privateKey: privateKey,
		address:    address.Hex(),
	}, nil
}

// Sign produces the X-Flashbots-Signature header value using personal_sign (EIP-191).
// Format: address:signature_hex
// The message signed is the keccak256 hash of the body, hex-encoded, wrapped in the personal_sign prefix.
func (s *Signer) Sign(body []byte) (string, error) {
	bodyHash := crypto.Keccak256Hash(body)
	hashHex := "0x" + hex.EncodeToString(bodyHash.Bytes())
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(hashHex), hashHex)
	hashToSign := crypto.Keccak256Hash([]byte(msg))
	signature, err := crypto.Sign(hashToSign.Bytes(), s.privateKey)
	if err != nil {
		return "", err
	}
	sigHex := hex.EncodeToString(signature)
	return fmt.Sprintf("%s:0x%s", s.address, sigHex), nil
}
