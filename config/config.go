package config

import (
	"crypto/tls"
	"fmt"
	"os"

	"go.temporal.io/sdk/client"
)

const (
	LocalAddress     = "localhost:7233"
	DefaultNamespace = "default"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func NewTemporalClient() (client.Client, error) {
	address := getEnv("TEMPORAL_QUIZ_ADDRESS", LocalAddress)
	namespace := getEnv("TEMPORAL_QUIZ_NAMESPACE", DefaultNamespace)
	apiKey := os.Getenv("TEMPORAL_QUIZ_API_KEY")
	certPath := os.Getenv("TEMPORAL_QUIZ_TLS_CERT_PATH")
	keyPath := os.Getenv("TEMPORAL_QUIZ_TLS_KEY_PATH")

	isCloud := address != LocalAddress

	opts := client.Options{
		HostPort:  address,
		Namespace: namespace,
	}

	if isCloud && apiKey != "" {
		opts.Credentials = client.NewAPIKeyStaticCredentials(apiKey)
		opts.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{},
		}
	} else if isCloud && certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("loading TLS cert/key: %w", err)
		}
		opts.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}
	}

	return client.Dial(opts)
}

// GetAnthropicKey returns the Anthropic API key from TEMPORAL_QUIZ_ANTHROPIC_API_KEY env var.
func GetAnthropicKey() string {
	return os.Getenv("TEMPORAL_QUIZ_ANTHROPIC_API_KEY")
}

// GetLLMModel returns the model to use, defaults to claude-sonnet-4-6.
func GetLLMModel() string {
	return getEnv("TEMPORAL_QUIZ_LLM_MODEL", "claude-sonnet-4-6")
}
