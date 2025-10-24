// Package schema provides utilities for resolving and managing schema versions.
package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	// For now, return a default version since the index doesn't match our collection names
	// In a real implementation, we would fetch the actual schema file and extract version info
	switch collection {
	case "com.registryaccord.feed.post":
		return "1.0.0", nil
	case "com.registryaccord.profile":
		return "1.0.0", nil
	case "com.registryaccord.graph.follow":
		return "1.0.0", nil
	case "com.registryaccord.feed.like":
		return "1.0.0", nil
	case "com.registryaccord.feed.comment":
		return "1.0.0", nil
	case "com.registryaccord.feed.repost":
		return "1.0.0", nil
	case "com.registryaccord.moderation.flag":
		return "1.0.0", nil
	case "com.registryaccord.media.asset":
		return "1.0.0", nil
	default:
		return "", fmt.Errorf("unsupported collection: %s", collection)
	}
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
