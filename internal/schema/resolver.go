// Package schema provides utilities for resolving and managing schema versions.
package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SchemaIndex represents the structure of SPEC_INDEX.json
type SchemaIndex struct {
	Schemas     []SchemaInfo `json:"schemas"`
	GeneratedAt time.Time    `json:"generatedAt"`
}

// SchemaInfo represents information about a schema
type SchemaInfo struct {
	Namespace     string  `json:"namespace"`
	Name          string  `json:"name"`
	Versions      []string `json:"versions"`
	LatestStable  string  `json:"latestStable"`
	Status        string  `json:"status"`
	Deprecates    *string `json:"deprecates"`
	ReplacedBy    *string `json:"replacedBy"`
}

// Resolver handles schema resolution from the specs repository
type Resolver struct {
	specsURL     string
	cacheDir     string
	index        *SchemaIndex
	lastUpdate   time.Time
	cacheTimeout time.Duration
}

// NewResolver creates a new schema resolver
func NewResolver(specsURL, cacheDir string) *Resolver {
	return &Resolver{
		specsURL:     specsURL,
		cacheDir:     cacheDir,
		cacheTimeout: 5 * time.Minute, // 5-minute cache
	}
}

// ResolveSchemaVersion resolves a collection NSID to its latest stable version
func (r *Resolver) ResolveSchemaVersion(collection string) (string, error) {
	// Load or refresh the schema index
	index, err := r.getSchemaIndex()
	if err != nil {
		return "", fmt.Errorf("failed to get schema index: %w", err)
	}

	// Convert collection NSID to the format used in the index
	// For example: com.registryaccord.feed.post -> ra.social.post
	parts := strings.Split(collection, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid collection NSID: %s", collection)
	}

	// Map the namespace
	var namespace string
	switch {
	case strings.HasPrefix(collection, "com.registryaccord.feed."):
		namespace = "ra.social." + parts[3]
	case strings.HasPrefix(collection, "com.registryaccord.graph."):
		namespace = "ra.social." + parts[3]
	case strings.HasPrefix(collection, "com.registryaccord.profile"):
		namespace = "ra.social.profile"
	case strings.HasPrefix(collection, "com.registryaccord.media."):
		namespace = "ra.social.media"
	case strings.HasPrefix(collection, "com.registryaccord.moderation."):
		namespace = "ra.social.moderation"
	default:
		return "", fmt.Errorf("unsupported collection namespace: %s", collection)
	}

	// Find the schema in the index
	for _, schema := range index.Schemas {
		if schema.Namespace == namespace {
			if schema.Status == "stable" {
				return schema.LatestStable, nil
			}
			
			// Check if this is a deprecated schema
			if schema.Deprecates != nil {
				// Return the version with a deprecation warning
				return schema.LatestStable + ":deprecated", nil
			}
			
			// If no stable version, use the latest version
			if len(schema.Versions) > 0 {
				version := schema.Versions[len(schema.Versions)-1]
				// If this schema is being replaced, mark it as deprecated
				if schema.ReplacedBy != nil {
					version += ":deprecated"
				}
				return version, nil
			}
			
			return "", fmt.Errorf("no versions found for schema %s", namespace)
		}
	}

	return "", fmt.Errorf("schema not found for collection: %s (namespace: %s)", collection, namespace)
}

// getSchemaIndex retrieves the schema index from the specs repository
func (r *Resolver) getSchemaIndex() (*SchemaIndex, error) {
	// Check if we have a cached version that's still valid
	if r.index != nil && time.Since(r.lastUpdate) < r.cacheTimeout {
		return r.index, nil
	}

	// Try to load from local cache first
	index, err := r.loadFromCache()
	if err == nil && index != nil && time.Since(index.GeneratedAt) < 24*time.Hour {
		// Valid cached index
		r.index = index
		r.lastUpdate = time.Now()
		return index, nil
	}

	// Fetch from remote repository
	index, err = r.fetchFromRemote()
	if err != nil {
		// If remote fetch fails but we have a stale cache, use it
		if r.index != nil {
			return r.index, nil
		}
		return nil, fmt.Errorf("failed to fetch schema index: %w", err)
	}

	// Update cache
	r.index = index
	r.lastUpdate = time.Now()
	r.saveToCache(index)

	return index, nil
}

// loadFromCache loads the schema index from local cache
func (r *Resolver) loadFromCache() (*SchemaIndex, error) {
	cachePath := filepath.Join(r.cacheDir, "SPEC_INDEX.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var index SchemaIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// saveToCache saves the schema index to local cache
func (r *Resolver) saveToCache(index *SchemaIndex) {
	// Ensure cache directory exists
	if err := os.MkdirAll(r.cacheDir, 0755); err != nil {
		return // Ignore cache errors
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return // Ignore cache errors
	}

	cachePath := filepath.Join(r.cacheDir, "SPEC_INDEX.json")
	_ = os.WriteFile(cachePath, data, 0644) // Ignore errors
}

// fetchFromRemote fetches the schema index from the remote specs repository
func (r *Resolver) fetchFromRemote() (*SchemaIndex, error) {
	indexURL := r.specsURL + "/SPEC_INDEX.json"
	resp, err := http.Get(indexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch schema index: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var index SchemaIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}
