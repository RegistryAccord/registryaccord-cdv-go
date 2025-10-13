// internal/identity/client.go
// Package identity provides a client for interacting with the RegistryAccord identity service.
// It handles DID resolution and validation for JWT authentication.
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Client for interacting with the RegistryAccord identity service.
// It provides methods for resolving and validating DIDs.
type Client struct {
	base string       // Base URL of the identity service
	hc   *http.Client // HTTP client with custom configuration
}

// Record represents an identity record from the identity service.
// It contains the DID and associated public key information.
type Record struct {
	DID       string `json:"did"`       // Decentralized Identifier
	PublicKey string `json:"publicKey"` // Public key for JWT verification
	CreatedAt string `json:"createdAt"` // When the identity was created
}

// ErrNotFound is returned when an identity record is not found.
var ErrNotFound = errors.New("identity not found")

// New creates a new identity client with the specified base URL.
// It configures appropriate timeouts for identity service requests.
// Parameters:
//   - baseURL: Base URL of the identity service
// Returns:
//   - *Client: Initialized identity client
func New(baseURL string) *Client {
	// Configure HTTP transport with connection timeouts
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
	}
	
	// Create HTTP client with request timeout
	return &Client{
		base: baseURL,
		hc:   &http.Client{Transport: transport, Timeout: 3 * time.Second},
	}
}

// Get retrieves an identity record for the specified DID.
// It makes an HTTP request to the identity service to resolve the DID.
// Parameters:
//   - ctx: Context for the request
//   - did: Decentralized Identifier to resolve
// Returns:
//   - Record: Identity record if found
//   - error: ErrNotFound if record doesn't exist, or other error
func (c *Client) Get(ctx context.Context, did string) (Record, error) {
	// Construct the request URL
	u, _ := url.Parse(c.base)
	u.Path = "/xrpc/com.registryaccord.identity.get"
	q := u.Query()
	q.Set("did", did)
	u.RawQuery = q.Encode()

	// Create and execute the HTTP request
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	resp, err := c.hc.Do(req)
	if err != nil {
		return Record{}, err
	}
	defer resp.Body.Close()

	// Handle different response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Parse successful response
		var rec Record
		if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
			return Record{}, err
		}
		return rec, nil
	case http.StatusNotFound:
		// DID not found
		return Record{}, ErrNotFound
	default:
		// Other error
		return Record{}, fmt.Errorf("identity get failed: %s", resp.Status)
	}
}
