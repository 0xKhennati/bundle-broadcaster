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

func (s *Signer) Sign(body []byte) (string, error) {
	hash := crypto.Keccak256Hash(body)
	signature, err := crypto.Sign(hash.Bytes(), s.privateKey)
	if err != nil {
		return "", err
	}
	sigHex := hex.EncodeToString(signature)
	return fmt.Sprintf("0x%s:%s", s.address, sigHex), nil
}
