// Package conformance provides conformance tests for CDV implementation.
package conformance

import (
	"testing"
)

// TestConformance runs the full conformance test suite.
func TestConformance(t *testing.T) {
	// Create harness with default configuration
	cfg := Config{
		UsePostgres:            false,
		UseNATS:                false,
		JWTIssuer:              "test-issuer",
		JWTAudience:            "test-audience",
		SpecsURL:               "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas",
		RejectDeprecatedSchemas: false,
	}
	
	harness, err := NewHarness(cfg)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}
	defer harness.Close()
	
	// Run conformance tests
	t.Run("Conformance", func(t *testing.T) {
		harness.RunConformanceTests(t)
	})
	
	// Run acceptance tests
	t.Run("Acceptance", func(t *testing.T) {
		harness.RunAcceptanceTests(t)
	})
}
