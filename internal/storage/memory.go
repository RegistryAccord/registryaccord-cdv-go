// internal/storage/memory.go
// Package storage provides implementations of the Store interface
// for both in-memory and PostgreSQL storage backends.
package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/RegistryAccord/registryaccord-cdv-go/internal/model"
)

// Standard errors returned by the storage layer
var (
	ErrNotFound = errors.New("not found")  // Returned when a record is not found
	ErrConflict  = errors.New("conflict")   // Returned when a record already exists
)

// Store interface defines the storage operations required by the CDV service.
// This interface is implemented by both in-memory and PostgreSQL storage backends.
type Store interface {
	// Record operations for managing user-generated content
	CreateRecord(ctx context.Context, record model.Record) error                    // Create a new record
	ListRecords(ctx context.Context, query model.ListRecordsQuery) (*model.ListRecordsResult, error) // List records with filtering
	GetRecordByURI(ctx context.Context, uri string) (*model.Record, error)         // Get a record by its URI
	
	// Media operations for managing media assets
	CreateMediaAsset(ctx context.Context, asset model.MediaAsset) error            // Create a new media asset
	GetMediaAsset(ctx context.Context, assetId string) (*model.MediaAsset, error)  // Get a media asset by ID
	UpdateMediaAsset(ctx context.Context, asset model.MediaAsset) error            // Update an existing media asset
	
	// Account operations for managing user accounts
	CreateAccount(ctx context.Context, did string) error                           // Create a new account
	GetAccount(ctx context.Context, did string) (*model.Account, error)            // Get an account by DID
	
	// Idempotency operations
	StoreIdempotentResponse(ctx context.Context, keyHash string, responseBody []byte, statusCode int, expiresAt time.Time) error // Store idempotent response
	GetIdempotentResponse(ctx context.Context, keyHash string) ([]byte, int, error) // Get cached idempotent response
}

// IdempotentResponse represents a cached idempotent response
type IdempotentResponse struct {
	ResponseBody []byte    // Cached response body
	StatusCode   int       // HTTP status code
	ExpiresAt    time.Time // When the entry expires
}

// memory implements the Store interface using in-memory storage.
// It's intended for development and testing purposes.
type memory struct {
	mu         sync.RWMutex              // Protects concurrent access to maps
	accounts   map[string]*model.Account // Map of DID to account
	records    map[string]*model.Record  // Map of URI to record
	mediaAssets map[string]*model.MediaAsset // Map of asset ID to media asset
	recordsByDID map[string][]*model.Record // Map of DID to records for efficient listing
	idempotency map[string]*IdempotentResponse // Map of key hash to idempotent responses
}

// NewMemory creates a new in-memory storage implementation.
// Returns a Store interface that can be used for testing or development.
func NewMemory() Store {
	return &memory{
		accounts:     make(map[string]*model.Account),
		records:      make(map[string]*model.Record),
		mediaAssets:  make(map[string]*model.MediaAsset),
		recordsByDID: make(map[string][]*model.Record),
		idempotency:  make(map[string]*IdempotentResponse),
	}
}

func (m *memory) CreateAccount(ctx context.Context, did string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.accounts[did]; exists {
		return ErrConflict
	}
	
	m.accounts[did] = &model.Account{
		DID:       did,
		CreatedAt: time.Now().UTC(),
	}
	return nil
}

func (m *memory) GetAccount(ctx context.Context, did string) (*model.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	account, exists := m.accounts[did]
	if !exists {
		return nil, ErrNotFound
	}
	return account, nil
}

func (m *memory) CreateRecord(ctx context.Context, record model.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if account exists
	if _, exists := m.accounts[record.DID]; !exists {
		return errors.New("account not found")
	}
	
	// Check if record already exists
	if _, exists := m.records[record.URI]; exists {
		return ErrConflict
	}
	
	// Store the record
	recordCopy := record
	m.records[record.URI] = &recordCopy
	m.recordsByDID[record.DID] = append(m.recordsByDID[record.DID], &recordCopy)
	return nil
}

// encodeMemoryCursor encodes cursor data into a base64 string for memory storage
func encodeMemoryCursor(lastIndexedAt time.Time, lastRKey string) string {
	data := map[string]interface{}{
		"lastIndexedAt": lastIndexedAt.UnixNano(),
		"lastRKey":      lastRKey,
	}
	jsonBytes, _ := json.Marshal(data)
	return base64.URLEncoding.EncodeToString(jsonBytes)
}

// decodeMemoryCursor decodes a base64 cursor string into cursor data for memory storage
func decodeMemoryCursor(cursor string) (time.Time, string, error) {
	dataBytes, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return time.Time{}, "", err
	}
	
	lastIndexedAt := time.Unix(0, int64(data["lastIndexedAt"].(float64)))
	lastRKey := data["lastRKey"].(string)
	
	return lastIndexedAt, lastRKey, nil
}

func (m *memory) ListRecords(ctx context.Context, query model.ListRecordsQuery) (*model.ListRecordsResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	records, exists := m.recordsByDID[query.DID]
	if !exists {
		return &model.ListRecordsResult{Records: []model.Record{}}, nil
	}
	
	// Filter by collection if specified
	filtered := make([]*model.Record, 0)
	for _, record := range records {
		if query.Collection == "" || record.Collection == query.Collection {
			filtered = append(filtered, record)
		}
	}
	// Sort by indexedAt descending, then by RKey ascending for stable ordering
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IndexedAt.Equal(filtered[j].IndexedAt) {
			return filtered[i].RKey < filtered[j].RKey
		}
		return filtered[i].IndexedAt.After(filtered[j].IndexedAt)
	})
	
	// Apply cursor if provided
	startIndex := 0
	if query.Cursor != "" {
		lastIndexedAt, lastRKey, err := decodeMemoryCursor(query.Cursor)
		if err == nil {
			// Find the starting position based on cursor
			for i, record := range filtered {
				if record.IndexedAt.Before(lastIndexedAt) || 
				   (record.IndexedAt.Equal(lastIndexedAt) && record.RKey > lastRKey) {
					startIndex = i + 1
					break
				}
			}
		}
	}
	
	// Apply limit
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}
	
	// Calculate end index
	endIndex := startIndex + limit
	if endIndex > len(filtered) {
		endIndex = len(filtered)
	}
	
	// Extract the page of records
	filtered = filtered[startIndex:endIndex]
	
	// Convert to result format
	resultRecords := make([]model.Record, len(filtered))
	for i, record := range filtered {
		resultRecords[i] = *record
	}
	
	result := &model.ListRecordsResult{
		Records: resultRecords,
	}
	
	// Add next cursor if there are more records
	if endIndex < len(records) && len(resultRecords) > 0 {
		lastRecord := resultRecords[len(resultRecords)-1]
		result.NextCursor = encodeMemoryCursor(lastRecord.IndexedAt, lastRecord.RKey)
	}
	
	return result, nil
}

func (m *memory) GetRecordByURI(ctx context.Context, uri string) (*model.Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	record, exists := m.records[uri]
	if !exists {
		return nil, ErrNotFound
	}
	return record, nil
}

func (m *memory) CreateMediaAsset(ctx context.Context, asset model.MediaAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if account exists
	if _, exists := m.accounts[asset.DID]; !exists {
		return errors.New("account not found")
	}
	
	// Check if asset already exists
	if _, exists := m.mediaAssets[asset.AssetID]; exists {
		return ErrConflict
	}
	
	// Store the asset
	assetCopy := asset
	m.mediaAssets[asset.AssetID] = &assetCopy
	return nil
}

func (m *memory) GetMediaAsset(ctx context.Context, assetId string) (*model.MediaAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	asset, exists := m.mediaAssets[assetId]
	if !exists {
		return nil, ErrNotFound
	}
	return asset, nil
}

func (m *memory) UpdateMediaAsset(ctx context.Context, asset model.MediaAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if asset exists
	if _, exists := m.mediaAssets[asset.AssetID]; !exists {
		return ErrNotFound
	}
	
	// Update the asset
	assetCopy := asset
	m.mediaAssets[asset.AssetID] = &assetCopy
	return nil
}

// StoreIdempotentResponse stores an idempotent response in memory
func (m *memory) StoreIdempotentResponse(ctx context.Context, keyHash string, responseBody []byte, statusCode int, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	responseCopy := make([]byte, len(responseBody))
	copy(responseCopy, responseBody)
	
	m.idempotency[keyHash] = &IdempotentResponse{
		ResponseBody: responseCopy,
		StatusCode:   statusCode,
		ExpiresAt:    expiresAt,
	}
	return nil
}

// GetIdempotentResponse retrieves a cached idempotent response from memory
func (m *memory) GetIdempotentResponse(ctx context.Context, keyHash string) ([]byte, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	response, exists := m.idempotency[keyHash]
	if !exists {
		return nil, 0, ErrNotFound
	}
	
	// Check if the response has expired
	if time.Now().UTC().After(response.ExpiresAt) {
		// Remove expired entry
		delete(m.idempotency, keyHash)
		return nil, 0, ErrNotFound
	}
	
	responseCopy := make([]byte, len(response.ResponseBody))
	copy(responseCopy, response.ResponseBody)
	
	return responseCopy, response.StatusCode, nil
}
