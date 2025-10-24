// Package config provides tests for the configuration loading and management.
package config

import (
	"os"
	"testing"
)

// TestLoad tests the Load function with default values.
func TestLoad(t *testing.T) {
	// Clear environment variables that might affect the test
	os.Unsetenv("CDV_ENV")
	os.Unsetenv("CDV_PORT")
	os.Unsetenv("CDV_DB_DSN")
	os.Unsetenv("CDV_NATS_URL")
	os.Unsetenv("CDV_S3_ENDPOINT")
	os.Unsetenv("CDV_S3_REGION")
	os.Unsetenv("CDV_S3_BUCKET")
	os.Unsetenv("CDV_S3_ACCESS_KEY")
	os.Unsetenv("CDV_S3_SECRET_KEY")
	os.Unsetenv("CDV_JWT_ISSUER")
	os.Unsetenv("CDV_JWT_AUDIENCE")
	os.Unsetenv("IDENTITY_URL")
	
	// Set required JWT parameters for validation
	os.Setenv("CDV_JWT_ISSUER", "test-issuer")
	os.Setenv("CDV_JWT_AUDIENCE", "test-audience")
	
	// Clean up environment variables after test
	t.Cleanup(func() {
		os.Unsetenv("CDV_JWT_ISSUER")
		os.Unsetenv("CDV_JWT_AUDIENCE")
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check default values
	if cfg.Env != "dev" {
		t.Errorf("Load() Env = %v, want %v", cfg.Env, "dev")
	}
	if cfg.Port != "8080" {
		t.Errorf("Load() Port = %v, want %v", cfg.Port, "8080")
	}
	if cfg.S3Region != "us-east-1" {
		t.Errorf("Load() S3Region = %v, want %v", cfg.S3Region, "us-east-1")
	}
}

// TestLoadWithEnv tests the Load function with environment variables set.
func TestLoadWithEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("CDV_ENV", "test")
	os.Setenv("CDV_PORT", "9090")
	os.Setenv("CDV_DB_DSN", "postgres://test:test@localhost/test")
	os.Setenv("CDV_NATS_URL", "nats://localhost:4222")
	os.Setenv("CDV_S3_ENDPOINT", "http://localhost:9000")
	os.Setenv("CDV_S3_REGION", "us-west-2")
	os.Setenv("CDV_S3_BUCKET", "test-bucket")
	os.Setenv("CDV_S3_ACCESS_KEY", "test-access-key")
	os.Setenv("CDV_S3_SECRET_KEY", "test-secret-key")
	os.Setenv("CDV_JWT_ISSUER", "test-issuer")
	os.Setenv("CDV_JWT_AUDIENCE", "test-audience")
	os.Setenv("IDENTITY_URL", "http://localhost:8081")

	// Clean up environment variables after test
	t.Cleanup(func() {
		os.Unsetenv("CDV_ENV")
		os.Unsetenv("CDV_PORT")
		os.Unsetenv("CDV_DB_DSN")
		os.Unsetenv("CDV_NATS_URL")
		os.Unsetenv("CDV_S3_ENDPOINT")
		os.Unsetenv("CDV_S3_REGION")
		os.Unsetenv("CDV_S3_BUCKET")
		os.Unsetenv("CDV_S3_ACCESS_KEY")
		os.Unsetenv("CDV_S3_SECRET_KEY")
		os.Unsetenv("CDV_JWT_ISSUER")
		os.Unsetenv("CDV_JWT_AUDIENCE")
		os.Unsetenv("IDENTITY_URL")
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check values from environment variables
	if cfg.Env != "test" {
		t.Errorf("Load() Env = %v, want %v", cfg.Env, "test")
	}
	if cfg.Port != "9090" {
		t.Errorf("Load() Port = %v, want %v", cfg.Port, "9090")
	}
	if cfg.DatabaseDSN != "postgres://test:test@localhost/test" {
		t.Errorf("Load() DatabaseDSN = %v, want %v", cfg.DatabaseDSN, "postgres://test:test@localhost/test")
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("Load() NATSURL = %v, want %v", cfg.NATSURL, "nats://localhost:4222")
	}
	if cfg.S3Endpoint != "http://localhost:9000" {
		t.Errorf("Load() S3Endpoint = %v, want %v", cfg.S3Endpoint, "http://localhost:9000")
	}
	if cfg.S3Region != "us-west-2" {
		t.Errorf("Load() S3Region = %v, want %v", cfg.S3Region, "us-west-2")
	}
	if cfg.S3Bucket != "test-bucket" {
		t.Errorf("Load() S3Bucket = %v, want %v", cfg.S3Bucket, "test-bucket")
	}
	if cfg.S3AccessKey != "test-access-key" {
		t.Errorf("Load() S3AccessKey = %v, want %v", cfg.S3AccessKey, "test-access-key")
	}
	if cfg.S3SecretKey != "test-secret-key" {
		t.Errorf("Load() S3SecretKey = %v, want %v", cfg.S3SecretKey, "test-secret-key")
	}
	if cfg.JWTIssuer != "test-issuer" {
		t.Errorf("Load() JWTIssuer = %v, want %v", cfg.JWTIssuer, "test-issuer")
	}
	if cfg.JWTAudience != "test-audience" {
		t.Errorf("Load() JWTAudience = %v, want %v", cfg.JWTAudience, "test-audience")
	}
	if cfg.IdentityURL != "http://localhost:8081" {
		t.Errorf("Load() IdentityURL = %v, want %v", cfg.IdentityURL, "http://localhost:8081")
	}
}
