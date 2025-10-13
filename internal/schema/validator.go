// internal/schema/validator.go
// Package schema provides JSON schema validation for CDV records.
// It ensures that all records conform to their respective schemas before being stored.
package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// SupportedCollections lists all collections that are supported for schema validation.
// Only records belonging to these collections can be validated and stored.
var SupportedCollections = map[string]bool{
	"com.registryaccord.feed.post":     true,  // User posts/feed items
	"com.registryaccord.profile":       true,  // User profile information
	"com.registryaccord.graph.follow":  true,  // Follow relationships
	"com.registryaccord.feed.like":     true,  // Like interactions
	"com.registryaccord.feed.comment":  true,  // Comments on posts
	"com.registryaccord.feed.repost":   true,  // Reposts/retweets
	"com.registryaccord.moderation.flag": true,  // Moderation flags
	"com.registryaccord.media.asset":   true,  // Media assets
}

// SchemaVersions maps collection names to their current schema versions.
// This allows tracking which version of the schema was used for validation.
// Note: This is now dynamically resolved but kept for backward compatibility.
var SchemaVersions = map[string]string{
	"com.registryaccord.feed.post":     "1.0.0",  // Post schema version
	"com.registryaccord.profile":       "1.0.0",  // Profile schema version
	"com.registryaccord.graph.follow":  "1.0.0",  // Follow schema version
	"com.registryaccord.feed.like":     "1.0.0",  // Like schema version
	"com.registryaccord.feed.comment":  "1.0.0",  // Comment schema version
	"com.registryaccord.feed.repost":   "1.0.0",  // Repost schema version
	"com.registryaccord.moderation.flag": "1.0.0",  // Flag schema version
	"com.registryaccord.media.asset":   "1.0.0",  // Media asset schema version
}

// Validator validates records against JSON schemas.
// It ensures data integrity and consistency across all stored records.
type Validator struct {
	schemas map[string]*gojsonschema.Schema // Map of collection names to JSON schemas
	resolver *Resolver // Schema resolver for dynamic version resolution
}

// NewValidator creates a new schema validator.
// It initializes all supported schemas and prepares them for validation.
// Returns:
//   - *Validator: Initialized validator instance
//   - error: Any error that occurred during initialization
func NewValidator() (*Validator, error) {
	// Initialize the resolver
	resolver := NewResolver("https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas", "/tmp/registryaccord-specs-cache")
	
	// Initialize the validator with an empty schema map
	v := &Validator{
		schemas: make(map[string]*gojsonschema.Schema),
		resolver: resolver,
	}

	// Load all supported schemas
	if err := v.loadSchemas(); err != nil {
		return nil, fmt.Errorf("failed to load schemas: %w", err)
	}

	return v, nil
}

// SetResolver sets the schema resolver for dynamic version resolution
func (v *Validator) SetResolver(resolver *Resolver) {
	v.resolver = resolver
}

// loadSchemas loads all supported schemas.
// This function initializes the JSON schemas for all supported collection types.
// Each schema is loaded and compiled for efficient validation.
func (v *Validator) loadSchemas() error {
	// Load post schema - for user-generated content posts
	postSchema := `{"type":"object","required":["text","createdAt","authorDid"],"properties":{"text":{"type":"string","maxLength":2048},"createdAt":{"type":"string","format":"datetime"},"authorDid":{"type":"string","format":"did"}}}`
	if err := v.loadSchema("com.registryaccord.feed.post", postSchema); err != nil {
		return fmt.Errorf("failed to load post schema: %w", err)
	}

	// Load profile schema - for user profile information
	profileSchema := `{"type":"object","properties":{"displayName":{"type":"string","description":"The user's public display name.","maxLength":64},"bio":{"type":"string","description":"A short user biography.","maxLength":256}},"required":["displayName"]}`
	if err := v.loadSchema("com.registryaccord.profile", profileSchema); err != nil {
		return fmt.Errorf("failed to load profile schema: %w", err)
	}

	// Load follow schema - for follow relationships between users
	followSchema := `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string","format":"did"}}}`
	if err := v.loadSchema("com.registryaccord.graph.follow", followSchema); err != nil {
		return fmt.Errorf("failed to load follow schema: %w", err)
	}

	// Load like schema - for like interactions on content
	likeSchema := `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string"}}}`
	if err := v.loadSchema("com.registryaccord.feed.like", likeSchema); err != nil {
		return fmt.Errorf("failed to load like schema: %w", err)
	}

	// Load comment schema - for comments on posts
	commentSchema := `{"type":"object","required":["text","subject"],"properties":{"text":{"type":"string","maxLength":2048},"subject":{"type":"string"}}}`
	if err := v.loadSchema("com.registryaccord.feed.comment", commentSchema); err != nil {
		return fmt.Errorf("failed to load comment schema: %w", err)
	}

	// Load repost schema - for reposting/retweeting content
	repostSchema := `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string"}}}`
	if err := v.loadSchema("com.registryaccord.feed.repost", repostSchema); err != nil {
		return fmt.Errorf("failed to load repost schema: %w", err)
	}

	// Load moderation flag schema - for content moderation flags
	flagSchema := `{"type":"object","required":["subject","reason"],"properties":{"subject":{"type":"string"},"reason":{"type":"string","maxLength":256}}}`
	if err := v.loadSchema("com.registryaccord.moderation.flag", flagSchema); err != nil {
		return fmt.Errorf("failed to load flag schema: %w", err)
	}

	// Load media asset schema - for media file metadata
	mediaSchema := `{"type":"object","required":["mimeType","size","checksum"],"properties":{"mimeType":{"type":"string"},"size":{"type":"integer"},"checksum":{"type":"string"},"filename":{"type":"string"}}}`
	if err := v.loadSchema("com.registryaccord.media.asset", mediaSchema); err != nil {
		return fmt.Errorf("failed to load media schema: %w", err)
	}

	return nil
}

// loadSchema loads a single schema.
// It parses and compiles a JSON schema for a specific collection type.
// Parameters:
//   - collection: The collection name (e.g., "com.registryaccord.feed.post")
//   - schemaJSON: The JSON schema as a string
// Returns:
//   - error: Any error that occurred during schema loading
func (v *Validator) loadSchema(collection, schemaJSON string) error {
	// Create a loader for the schema JSON
	loader := gojsonschema.NewStringLoader(schemaJSON)
	
	// Compile the schema for efficient validation
	schema, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return fmt.Errorf("invalid schema for %s: %w", collection, err)
	}
	
	// Store the compiled schema
	v.schemas[collection] = schema
	return nil
}

// Validate validates a record against its schema.
// It ensures that the record conforms to the expected structure and constraints.
// Parameters:
//   - collection: The collection name (e.g., "com.registryaccord.feed.post")
//   - record: The record data to validate
// Returns:
//   - string: The schema version used for validation
//   - error: nil if valid, error with details if invalid
func (v *Validator) Validate(collection string, record map[string]interface{}) (string, error) {
	// Check if the collection is supported for validation
	if !SupportedCollections[collection] {
		return "", fmt.Errorf("unsupported collection: %s", collection)
	}

	// Get the compiled schema for this collection
	schema, exists := v.schemas[collection]
	if !exists {
		return "", fmt.Errorf("schema not found for collection: %s", collection)
	}

	// Convert the record to JSON for validation
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("failed to marshal record: %w", err)
	}

	// Perform the validation
	result, err := schema.Validate(gojsonschema.NewBytesLoader(recordJSON))
	if err != nil {
		return "", fmt.Errorf("validation error: %w", err)
	}

	// Check if validation failed and collect error details
	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return "", fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	// Get the schema version
	schemaVersion, exists := SchemaVersions[collection]
	if !exists {
		schemaVersion = "1.0.0" // Default version if not specified
	}

	// Record is valid
	return schemaVersion, nil
}

// ResolveSchemaVersion resolves a collection NSID to its latest stable version
func (v *Validator) ResolveSchemaVersion(collection string) (string, error) {
	return v.resolver.ResolveSchemaVersion(collection)
}
