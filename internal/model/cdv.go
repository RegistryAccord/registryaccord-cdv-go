// internal/model/cdv.go
// Package model defines the data structures used throughout the CDV service.
// These structures represent the core domain objects for accounts, records, and media assets.
package model

import (
	"time"
)

// Account represents a CDV account.
// Each account is identified by a Decentralized Identifier (DID) and tracks when it was created.
// This corresponds to the accounts table in storage.
type Account struct {
	DID       string    `json:"did" db:"did"`              // Decentralized Identifier (unique)
	CreatedAt time.Time `json:"createdAt" db:"created_at"`  // When the account was created
}

// Record represents a CDV record.
// A record is a piece of user-generated content that belongs to a specific collection.
// This corresponds to the records table in storage.
type Record struct {
	ID           string                 `json:"id" db:"id"`                    // Unique record identifier
	DID          string                 `json:"did" db:"did"`                  // Owner's Decentralized Identifier
	Collection   string                 `json:"collection" db:"collection"`    // Type of record (e.g., post, profile)
	RKey         string                 `json:"rkey" db:"rkey"`                // Record key for uniqueness
	URI          string                 `json:"uri" db:"uri"`                  // Unique resource identifier
	CID          string                 `json:"cid" db:"cid"`                  // Content identifier (hash)
	Value        map[string]interface{} `json:"value" db:"value"`              // Record data as JSON
	IndexedAt    time.Time              `json:"indexedAt" db:"indexed_at"`     // When the record was indexed
	SchemaVersion string                `json:"schemaVersion" db:"schema_version"` // Schema version for validation
}

// MediaAsset represents a CDV media asset.
// A media asset is a file (image, video, etc.) that has been uploaded and processed.
// This corresponds to the media_assets table in storage.
type MediaAsset struct {
	AssetID   string    `json:"assetId" db:"asset_id"`    // Unique asset identifier
	DID       string    `json:"did" db:"did"`              // Owner's Decentralized Identifier
	URI       string    `json:"uri" db:"uri"`              // Unique resource identifier
	MimeType  string    `json:"mimeType" db:"mime_type"`   // MIME type of the media file
	Size      int64     `json:"size" db:"size"`            // Size in bytes
	Checksum  string    `json:"checksum" db:"checksum"`    // SHA-256 checksum for integrity
	CreatedAt time.Time `json:"createdAt" db:"created_at"`  // When the asset was created
}

// OperationLogEntry represents an entry in the operation log.
// This provides an audit trail of all operations performed in the system.
// This corresponds to the op_log table in storage.
type OperationLogEntry struct {
	Sequence    int64                  `json:"sequence" db:"seq"`         // Sequential operation ID
	Type        string                 `json:"type" db:"type"`             // Type of operation performed
	Reference   string                 `json:"reference" db:"ref"`         // Reference to affected record
	DID         string                 `json:"did" db:"did"`               // User who performed operation
	Payload     map[string]interface{} `json:"payload" db:"payload"`       // Operation details
	OccurredAt  time.Time              `json:"occurredAt" db:"occurred_at"` // When operation occurred
}

// ListRecordsQuery represents the query parameters for listing records.
// It allows filtering and pagination when retrieving records.
type ListRecordsQuery struct {
	DID        string    `json:"did"`        // Filter by owner's DID
	Collection string    `json:"collection"` // Filter by collection type
	Limit      int       `json:"limit"`      // Maximum number of records to return
	Cursor     string    `json:"cursor"`     // Pagination cursor
	Since      time.Time `json:"since"`      // Filter records created after this time
	Until      time.Time `json:"until"`      // Filter records created before this time
}

// ListRecordsResult represents the result of listing records.
// It includes the records and pagination information.
type ListRecordsResult struct {
	Records    []Record `json:"records"`              // List of records matching the query
	NextCursor string   `json:"nextCursor,omitempty"` // Cursor for next page of results
}

// CreateRecordRequest represents the request body for creating a record.
// It contains all the information needed to create a new record.
type CreateRecordRequest struct {
	Collection      string                 `json:"collection"`       // Type of record to create
	DID             string                 `json:"did"`              // Owner's Decentralized Identifier
	Record          map[string]interface{} `json:"record"`           // Record data
	CreatedAt       *time.Time             `json:"createdAt,omitempty"` // Optional creation time
	IdempotencyKey  string                 `json:"idempotencyKey,omitempty"` // Key for idempotent operations
}

// CreateRecordResponse represents the response body for creating a record.
// It follows the standard API response format with a data wrapper.
type CreateRecordResponse struct {
	Data CreateRecordData `json:"data"` // Record creation result
}

// CreateRecordData contains the details of a successfully created record.
type CreateRecordData struct {
	URI       string    `json:"uri"`       // Unique resource identifier of the new record
	CID       string    `json:"cid"`       // Content identifier (hash) of the record
	IndexedAt time.Time `json:"indexedAt"` // When the record was indexed
}

// UploadInitRequest represents the request body for initializing a media upload.
// It contains the metadata needed to prepare for media file upload.
type UploadInitRequest struct {
	DID      string `json:"did"`      // Owner's Decentralized Identifier
	MimeType string `json:"mimeType"` // MIME type of the file to be uploaded
	Size     int64  `json:"size"`     // Size of the file in bytes
	SHA256   string `json:"sha256,omitempty"` // Optional SHA-256 checksum for integrity
	Filename string `json:"filename,omitempty"` // Optional original filename
}

// UploadInitResponse represents the response body for initializing a media upload.
// It provides the information needed to actually upload the file.
type UploadInitResponse struct {
	Data UploadInitData `json:"data"` // Upload initialization result
}

// UploadInitData contains the details needed to upload a media file.
type UploadInitData struct {
	AssetID   string    `json:"assetId"`   // Unique identifier for the media asset
	UploadURL string    `json:"uploadUrl"` // Presigned URL for uploading the file
	ExpiresAt time.Time `json:"expiresAt"` // When the upload URL expires
}

// FinalizeRequest represents the request body for finalizing a media upload.
// It contains the checksum verification needed to complete the upload process.
type FinalizeRequest struct {
	AssetID string `json:"assetId"` // Identifier of the media asset being finalized
	SHA256  string `json:"sha256"`  // SHA-256 checksum for integrity verification
}

// FinalizeResponse represents the response body for finalizing a media upload.
// It returns the complete media asset metadata after successful finalization.
type FinalizeResponse struct {
	Data MediaAsset `json:"data"` // Finalized media asset metadata
}

// GetMediaMetaResponse represents the response body for getting media metadata.
// It returns the metadata for a specific media asset.
type GetMediaMetaResponse struct {
	Data MediaAsset `json:"data"` // Requested media asset metadata
}
