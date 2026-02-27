package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type RelayConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

type Config struct {
	Server    ServerConfig `json:"server"`
	Auth      AuthConfig   `json:"auth"`
	PrivateKey string      `json:"private_key"`
	Relays    []RelayConfig `json:"relays"`
}

type AuthConfig struct {
	PasswordHash    string `json:"password_hash"`
	MaxAttempts     int    `json:"max_attempts"`
	LockoutMinutes  int    `json:"lockout_minutes"`
}

type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

func (s *ServerConfig) Addr() string {
	addr := s.Address
	if addr == "" {
		addr = "0.0.0.0"
	}
	port := s.Port
	if port == 0 {
		port = 1111
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

const (
	RelayTypeDefault       = "default"
	RelayTypeEndOfBlock    = "sendEndOfBlockBundle"
	RelayTypeUnifiedBundle = "unified_bundle"
)

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
