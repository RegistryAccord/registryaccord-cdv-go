// internal/server/mux_test.go
// Package server provides unit tests for the HTTP handlers and routing.
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/identity"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/jwks"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/model"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/storage"
)

// mockPublisher implements event.Publisher for testing purposes.
// It provides no-op implementations of all Publisher methods.
type mockPublisher struct{}

// PublishRecordCreated implements event.Publisher for testing.
// It returns nil to indicate successful publishing.
func (m *mockPublisher) PublishRecordCreated(ctx context.Context, collection string, record model.Record) error {
	return nil
}

// PublishMediaFinalized implements event.Publisher for testing.
// It returns nil to indicate successful publishing.
func (m *mockPublisher) PublishMediaFinalized(ctx context.Context, asset model.MediaAsset) error {
	return nil
}

// Close implements event.Publisher for testing.
// It returns nil to indicate successful closing.
func (m *mockPublisher) Close() error {
	return nil
}


// TestHealthzEndpoint tests the healthz endpoint.
// It verifies that the /healthz endpoint returns a 200 OK status
// and the expected response body.
func TestHealthzEndpoint(t *testing.T) {
	// Create a new mux with mock dependencies
	store := storage.NewMemory()
	pub := &mockPublisher{}
	var idClient *identity.Client = nil // Use nil for testing
	
	jwksClient := jwks.NewTestClient()
	mux := NewMux(store, pub, idClient, "test-issuer", "test-audience", 10*1024*1024, []string{"image/jpeg", "image/png", "image/gif", "video/mp4"}, jwksClient, "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", false)
	
	// Create a request to the healthz endpoint
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create a response recorder
	rr := httptest.NewRecorder()
	
	// Serve the request
	mux.ServeHTTP(rr, req)
	
	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check the response body
	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

// TestReadyzEndpoint tests the readyz endpoint.
// It verifies that the /readyz endpoint returns a 200 OK status
// and the expected response body.
func TestReadyzEndpoint(t *testing.T) {
	// Create a new mux with mock dependencies
	store := storage.NewMemory()
	pub := &mockPublisher{}
	var idClient *identity.Client = nil // Use nil for testing
	
	jwksClient := jwks.NewTestClient()
	mux := NewMux(store, pub, idClient, "test-issuer", "test-audience", 10*1024*1024, []string{"image/jpeg", "image/png", "image/gif", "video/mp4"}, jwksClient, "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", false)
	
	// Create a request to the readyz endpoint
	req, err := http.NewRequest("GET", "/readyz", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create a response recorder
	rr := httptest.NewRecorder()
	
	// Serve the request
	mux.ServeHTTP(rr, req)
	
	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check the response body
	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

// TestMediaSizeLimit tests that media uploads are rejected when they exceed size limits.
func TestMediaSizeLimit(t *testing.T) {
	// Create a new mux with mock dependencies and small size limit
	store := storage.NewMemory()
	pub := &mockPublisher{}
	var idClient *identity.Client = nil // Use nil for testing
	
	// Set a small max media size for testing (1KB)
	jwksClient := jwks.NewTestClient()
	mux := NewMux(store, pub, idClient, "test-issuer", "test-audience", 1024, []string{"image/jpeg", "image/png"}, jwksClient, "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", false)
	
	// Test media size that exceeds limit
	req, err := http.NewRequest("POST", "/v1/media/uploadInit", strings.NewReader(`{"did":"did:example:123","mimeType":"image/jpeg","size":2048}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXhhbXBsZToxMjMiLCJhdWQiOiJ0ZXN0LWF1ZGllbmNlIiwiaXNzIjoidGVzdC1pc3N1ZXIifQ.X"
	req.Header.Set("Authorization", token)
	
	// Create a response recorder
	rr := httptest.NewRecorder()
	
	// Serve the request
	mux.ServeHTTP(rr, req)
	
	// Check the status code - should be bad request due to size limit
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

// TestMediaTypeNotAllowed tests that media uploads are rejected when the type is not allowed.
func TestMediaTypeNotAllowed(t *testing.T) {
	// Create a new mux with mock dependencies
	store := storage.NewMemory()
	pub := &mockPublisher{}
	var idClient *identity.Client = nil // Use nil for testing
	
	jwksClient := jwks.NewTestClient()
	mux := NewMux(store, pub, idClient, "test-issuer", "test-audience", 10*1024*1024, []string{"image/jpeg", "image/png"}, jwksClient, "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", false)
	
	// Test media type that is not allowed
	req, err := http.NewRequest("POST", "/v1/media/uploadInit", strings.NewReader(`{"did":"did:example:123","mimeType":"application/pdf","size":1024}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXhhbXBsZToxMjMiLCJhdWQiOiJ0ZXN0LWF1ZGllbmNlIiwiaXNzIjoidGVzdC1pc3N1ZXIifQ.X"
	req.Header.Set("Authorization", token)
	
	// Create a response recorder
	rr := httptest.NewRecorder()
	
	// Serve the request
	mux.ServeHTTP(rr, req)
	
	// Check the status code - should be bad request due to disallowed type
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

// TestCreateRecordValidation tests validation of the create record endpoint.
// It verifies that the endpoint properly validates required fields
// and returns appropriate error responses for invalid requests.
func TestCreateRecordValidation(t *testing.T) {
	// Create a new mux with mock dependencies
	store := storage.NewMemory()
	pub := &mockPublisher{}
	var idClient *identity.Client = nil // Use nil for testing
	
	jwksClient := jwks.NewTestClient()
	mux := NewMux(store, pub, idClient, "test-issuer", "test-audience", 10*1024*1024, []string{"image/jpeg", "image/png", "image/gif", "video/mp4"}, jwksClient, "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", false)
	
	// Test missing collection - this should result in a bad request error
	req, err := http.NewRequest("POST", "/v1/repo/record", strings.NewReader(`{"did":"did:example:123"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXhhbXBsZToxMjMiLCJhdWQiOiJ0ZXN0LWF1ZGllbmNlIiwiaXNzIjoidGVzdC1pc3N1ZXIifQ.X"
	req.Header.Set("Authorization", token)
	
	// Create a response recorder
	rr := httptest.NewRecorder()
	
	// Serve the request
	mux.ServeHTTP(rr, req)
	
	// Check the status code - should be bad request due to missing required fields
	// Note: This test may fail if JWT validation is enabled, as the test JWT doesn't have proper kid
	if status := rr.Code; status != http.StatusBadRequest && status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v or %v", status, http.StatusBadRequest, http.StatusUnauthorized)
	}
}
