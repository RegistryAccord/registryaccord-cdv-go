// internal/event/nats.go
// Package event provides NATS JetStream implementation for event publishing.
// It streams record and media events to support real-time updates and audit trails.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RegistryAccord/registryaccord-cdv-go/internal/model"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Publisher interface defines the event publishing operations required by the CDV service.
// It provides methods for publishing record and media events to the event stream.
type Publisher interface {
	// Record events
	PublishRecordCreated(ctx context.Context, collection string, record model.Record) error
	
	// Media events
	PublishMediaFinalized(ctx context.Context, asset model.MediaAsset) error
	
	// Close closes the publisher connection
	Close() error
}

// noop is a no-op implementation of Publisher for when NATS is not configured.
// It implements all Publisher methods but does nothing, allowing the service
// to function without event streaming when NATS is not available.
type noop struct{}

// Close implements Publisher
// It does nothing and always returns nil.
func (n *noop) Close() error { return nil }

// PublishRecordCreated implements Publisher
// It does nothing and always returns nil.
func (n *noop) PublishRecordCreated(ctx context.Context, collection string, record model.Record) error { 
	return nil 
}

// PublishMediaFinalized implements Publisher
// It does nothing and always returns nil.
func (n *noop) PublishMediaFinalized(ctx context.Context, asset model.MediaAsset) error { 
	return nil 
}

// natsPub is the NATS JetStream implementation of Publisher.
// It connects to a NATS server and publishes events to JetStream streams.
type natsPub struct {
	nc *nats.Conn          // NATS connection
	js nats.JetStreamContext // JetStream context for stream operations
	
	// Deduplication fields
	recordDedup map[string]time.Time // Map of record IDs to last publish time
	mediaDedup  map[string]time.Time // Map of media asset IDs to last publish time
	mutex       sync.RWMutex         // Mutex for thread-safe access to dedup maps
}

// NewPublisherFromEnv creates a new publisher based on environment configuration.
// It reads the CDV_NATS_URL environment variable to determine if NATS should be used.
// If NATS is not configured or connection fails, it returns a no-op publisher.
// Returns:
//   - Publisher: Either a NATS publisher or a no-op publisher
func NewPublisherFromEnv() Publisher {
	// Check if NATS is configured
	url := os.Getenv("CDV_NATS_URL")
	if url == "" {
		return &noop{}
	}
	
	// Connect to NATS server
	nc, err := nats.Connect(url)
	if err != nil {
		slog.Warn("NATS connect failed, using noop publisher", "error", err)
		return &noop{}
	}
	
	// Create JetStream context for stream operations
	js, err := nc.JetStream()
	if err != nil {
		slog.Warn("NATS JetStream context creation failed, using noop publisher", "error", err)
		nc.Close()
		return &noop{}
	}
	
	// Initialize required streams
	if err := initStreams(js); err != nil {
		slog.Warn("NATS stream initialization failed, using noop publisher", "error", err)
		nc.Close()
		return &noop{}
	}
	
	return &natsPub{
		nc:          nc,
		js:          js,
		recordDedup: make(map[string]time.Time),
		mediaDedup:  make(map[string]time.Time),
	}
}

// initStreams initializes the required NATS streams.
// It creates the RA_RECORDS and RA_MEDIA streams with appropriate configurations.
// These streams are used for event streaming and audit trails.
func initStreams(js nats.JetStreamContext) error {
	// Create RA_RECORDS stream for record-related events
	// This stream handles all record creation and modification events
	_, err := js.AddStream(&nats.StreamConfig{
		Name:      "RA_RECORDS",               // Stream name
		Subjects:  []string{"cdv.records.*"},  // Subjects pattern for record events
		Retention: nats.LimitsPolicy,          // Retention policy
		MaxAge:    24 * time.Hour,             // Keep events for 24 hours
		Discard:   nats.DiscardOld,            // Discard old messages when limits reached
		Storage:   nats.FileStorage,           // Use file storage for persistence
	})
	if err != nil {
		return fmt.Errorf("failed to create RA_RECORDS stream: %w", err)
	}
	
	// Create RA_MEDIA stream for media-related events
	// This stream handles all media upload and processing events
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "RA_MEDIA",                 // Stream name
		Subjects:  []string{"cdv.media.*"},    // Subjects pattern for media events
		Retention: nats.LimitsPolicy,          // Retention policy
		MaxAge:    24 * time.Hour,             // Keep events for 24 hours
		Discard:   nats.DiscardOld,            // Discard old messages when limits reached
		Storage:   nats.FileStorage,           // Use file storage for persistence
	})
	if err != nil {
		return fmt.Errorf("failed to create RA_MEDIA stream: %w", err)
	}
	
	return nil
}

// EventEnvelope represents the standard event envelope structure.
// All events published to NATS are wrapped in this envelope for consistency.
type EventEnvelope struct {
	Type         string      `json:"type"`         // Event type identifier
	Version      string      `json:"version"`      // Event schema version
	OccurredAt   time.Time   `json:"occurredAt"`   // When the event occurred
	CorrelationID string     `json:"correlationId"` // Correlation ID for tracing
	Payload      interface{} `json:"payload"`      // Event-specific data
}

// Close closes the NATS connection.
// It gracefully closes the connection to the NATS server.
func (p *natsPub) Close() error {
	if p.nc != nil {
		p.nc.Close()
	}
	return nil
}

// shouldDedup checks if an event should be deduplicated based on the 2-minute window.
// It takes a map key (record ID or asset ID) and the dedup map, and returns true
// if the event should be deduplicated (i.e., it was published within the last 2 minutes).
func (p *natsPub) shouldDedup(key string, dedupMap map[string]time.Time) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if lastTime, exists := dedupMap[key]; exists {
		// Check if the last event was within the 2-minute dedup window
		return time.Since(lastTime) < 2*time.Minute
	}
	
	return false
}

// updateDedup updates the deduplication map with the current time for a given key.
// This should be called after successfully publishing an event.
func (p *natsPub) updateDedup(key string, dedupMap map[string]time.Time) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Clean up old entries to prevent memory leaks
	cutoff := time.Now().Add(-5 * time.Minute) // Keep entries for 5 minutes
	for k, t := range dedupMap {
		if t.Before(cutoff) {
			delete(dedupMap, k)
		}
	}
	
	// Update the current key with the current time
	dedupMap[key] = time.Now()
}

// PublishRecordCreated publishes a record created event.
// It wraps the record in an event envelope and publishes it to the RA_RECORDS stream.
// Parameters:
//   - ctx: Context for the operation
//   - collection: The record collection type
//   - record: The record that was created
// Returns:
//   - error: Any error that occurred during publishing
func (p *natsPub) PublishRecordCreated(ctx context.Context, collection string, record model.Record) error {
	// Check if this event should be deduplicated
	if p.shouldDedup(record.ID, p.recordDedup) {
		// Event was published recently, skip it
		return nil
	}
	
	// Create the subject name based on the collection
	subject := fmt.Sprintf("cdv.records.%s.created", collection)
	
	// Create the event envelope with metadata
	envelope := EventEnvelope{
		Type:         fmt.Sprintf("cdv.records.%s.created", collection), // Event type
		Version:      "1.0.0",                                           // Event schema version
		OccurredAt:   time.Now().UTC(),                                  // Event timestamp
		CorrelationID: uuid.New().String(),                              // Unique correlation ID
		Payload:      record,                                            // The record data
	}
	
	// Marshal the envelope to JSON
	b, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	
	// Publish the event to the stream
	_, err = p.js.Publish(subject, b)
	if err != nil {
		return err
	}
	
	// Update deduplication map on successful publish
	p.updateDedup(record.ID, p.recordDedup)
	
	return nil
}

// PublishMediaFinalized publishes a media finalized event.
// It wraps the media asset in an event envelope and publishes it to the RA_MEDIA stream.
// Parameters:
//   - ctx: Context for the operation
//   - asset: The media asset that was finalized
// Returns:
//   - error: Any error that occurred during publishing
func (p *natsPub) PublishMediaFinalized(ctx context.Context, asset model.MediaAsset) error {
	// Check if this event should be deduplicated
	if p.shouldDedup(asset.AssetID, p.mediaDedup) {
		// Event was published recently, skip it
		return nil
	}
	
	// Subject for media finalized events
	subject := "cdv.media.finalized"
	
	// Create the event envelope with metadata
	envelope := EventEnvelope{
		Type:         "cdv.media.finalized",      // Event type
		Version:      "1.0.0",                   // Event schema version
		OccurredAt:   time.Now().UTC(),          // Event timestamp
		CorrelationID: uuid.New().String(),      // Unique correlation ID
		Payload:      asset,                     // The media asset data
	}
	
	// Marshal the envelope to JSON
	b, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	
	// Publish the event to the stream
	_, err = p.js.Publish(subject, b)
	if err != nil {
		return err
	}
	
	// Update deduplication map on successful publish
	p.updateDedup(asset.AssetID, p.mediaDedup)
	
	return nil
}
