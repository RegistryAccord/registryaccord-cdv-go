// internal/storage/postgres.go
// Package storage provides PostgreSQL implementation of the Store interface.
// This implementation is intended for production use with persistent data storage.
package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RegistryAccord/registryaccord-cdv-go/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// It provides persistent storage for accounts, records, and media assets.
type postgres struct {
	db *pgxpool.Pool // Connection pool to PostgreSQL database
}

// NewPostgres creates a new PostgreSQL storage implementation.
// It establishes a connection pool to the database and initializes the schema.
// Parameters:
//   - dsn: Database connection string in PostgreSQL format
// Returns:
//   - Store: Implementation of the storage interface
//   - error: Any error that occurred during initialization
func NewPostgres(dsn string) (Store, error) {
	// Parse the database connection string
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database DSN: %w", err)
	}

	// Configure connection pool settings for optimal performance
	// Maximum number of connections
	config.MaxConns = 20
	// Minimum number of connections
	config.MinConns = 5
	// Maximum lifetime of a connection
	config.MaxConnLifetime = time.Hour
	// Maximum idle time before closing
	config.MaxConnIdleTime = time.Minute * 30
	// How often to check connection health
	config.HealthCheckPeriod = time.Minute

	// Establish connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize database schema
	if err := initSchema(ctx, pool); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &postgres{db: pool}, nil
}

// initSchema initializes the database schema.
// It creates all required tables and indexes if they don't already exist.
// This function is called automatically when creating a new PostgreSQL store.
func initSchema(ctx context.Context, db *pgxpool.Pool) error {
	// SQL schema definition with all required tables and indexes
	schema := `
		-- Accounts table for storing user accounts
		CREATE TABLE IF NOT EXISTS accounts (
		    did TEXT PRIMARY KEY,                    -- Decentralized Identifier
		    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()  -- Account creation time
		);

		-- Records table for storing user-generated content
		CREATE TABLE IF NOT EXISTS records (
		    id TEXT PRIMARY KEY,                     -- Unique record identifier
		    did TEXT NOT NULL REFERENCES accounts(did),  -- Owner's DID
		    collection TEXT NOT NULL,                -- Record collection type
		    rkey TEXT NOT NULL,                      -- Record key
		    uri TEXT NOT NULL UNIQUE,                -- Unique record URI
		    cid TEXT NOT NULL,                       -- Content identifier
		    value JSONB NOT NULL,                    -- Record data in JSON format
		    indexed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- Indexing time
		    schema_version TEXT NOT NULL,            -- Schema version for validation
		    UNIQUE(did, collection, rkey)            -- Prevent duplicate records
		);

		-- Indexes for records table to improve query performance
		CREATE INDEX IF NOT EXISTS idx_records_did_collection_indexed_at ON records(did, collection, indexed_at DESC);
		CREATE INDEX IF NOT EXISTS idx_records_cid ON records(cid);
		CREATE INDEX IF NOT EXISTS idx_records_indexed_at ON records(indexed_at DESC);

		-- Media assets table for storing media metadata
		CREATE TABLE IF NOT EXISTS media_assets (
		    asset_id TEXT PRIMARY KEY,               -- Unique asset identifier
		    did TEXT NOT NULL REFERENCES accounts(did),  -- Owner's DID
		    uri TEXT NOT NULL UNIQUE,                -- Unique asset URI
		    mime_type TEXT NOT NULL,                 -- MIME type of the media
		    size BIGINT NOT NULL,                    -- Size in bytes
		    checksum TEXT NOT NULL,                  -- SHA-256 checksum
		    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- Creation time
		    UNIQUE(did, asset_id)                    -- Prevent duplicate assets
		);

		-- Idempotency table for storing idempotency keys
		CREATE TABLE IF NOT EXISTS idempotency (
		    key_hash TEXT,                           -- Hash of the idempotency key
		    request_hash TEXT NOT NULL,              -- Hash of the request payload for conflict detection
		    response_body BYTEA NOT NULL,            -- Cached response body
		    response_status INTEGER NOT NULL,        -- HTTP status code
		    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- When the entry was created
		    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,  -- When the entry expires
		    PRIMARY KEY (key_hash, request_hash),    -- Composite primary key for conflict detection
		    UNIQUE(key_hash, request_hash)           -- Prevent conflicts with same key but different payloads
		);

		-- Index for idempotency table to improve query performance
		CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at ON idempotency(expires_at);

		-- Operation log table (append-only) for audit trail
		CREATE TABLE IF NOT EXISTS op_log (
		    seq BIGSERIAL PRIMARY KEY,               -- Sequential operation ID
		    type TEXT NOT NULL,                      -- Operation type
		    ref TEXT NOT NULL,                       -- Reference to affected record
		    did TEXT NOT NULL REFERENCES accounts(did),  -- User who performed operation
		    payload JSONB NOT NULL,                  -- Operation details
		    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()  -- When operation occurred
		);

		-- Indexes for op_log table to improve query performance
		CREATE INDEX IF NOT EXISTS idx_op_log_did ON op_log(did);
		CREATE INDEX IF NOT EXISTS idx_op_log_type ON op_log(type);
		CREATE INDEX IF NOT EXISTS idx_op_log_occurred_at ON op_log(occurred_at);
	`

	// Execute the schema creation SQL
	_, err := db.Exec(ctx, schema)
	return err
}

// Close closes the database connection pool
func (p *postgres) Close() {
	p.db.Close()
}

// CreateAccount creates a new account in the database
func (p *postgres) CreateAccount(ctx context.Context, did string) error {
	query := `INSERT INTO accounts (did, created_at) VALUES ($1, $2)`
	_, err := p.db.Exec(ctx, query, did, time.Now().UTC())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

// GetAccount retrieves an account by DID
func (p *postgres) GetAccount(ctx context.Context, did string) (*model.Account, error) {
	query := `SELECT did, created_at FROM accounts WHERE did = $1`
	var account model.Account
	
	err := p.db.QueryRow(ctx, query, did).Scan(&account.DID, &account.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	
	return &account, nil
}

// CreateRecord creates a new record in the database
func (p *postgres) CreateRecord(ctx context.Context, record model.Record) error {
	// First check if account exists
	if _, err := p.GetAccount(ctx, record.DID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("account not found: %s", record.DID)
		}
		return fmt.Errorf("failed to check account: %w", err)
	}

	// Convert value map to JSON
	valueJSON, err := json.Marshal(record.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal record value: %w", err)
	}

	query := `INSERT INTO records (id, did, collection, rkey, uri, cid, value, indexed_at, schema_version) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	
	_, err = p.db.Exec(ctx, query, 
		record.ID, 
		record.DID, 
		record.Collection, 
		record.RKey, 
		record.URI, 
		record.CID, 
		valueJSON, 
		record.IndexedAt, 
		record.SchemaVersion)
	
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("failed to create record: %w", err)
	}
	
	return nil
}

// cursorData represents the data encoded in a pagination cursor
type cursorData struct {
	LastIndexedAt time.Time // Timestamp of the last record
	LastRKey      string    // RKey of the last record
}

// encodeCursor encodes cursor data into a base64 string
func encodeCursor(lastIndexedAt time.Time, lastRKey string) string {
	data := cursorData{
		LastIndexedAt: lastIndexedAt,
		LastRKey:      lastRKey,
	}
	jsonBytes, _ := json.Marshal(data)
	return base64.URLEncoding.EncodeToString(jsonBytes)
}

// decodeCursor decodes a base64 cursor string into cursor data
func decodeCursor(cursor string) (*cursorData, error) {
	dataBytes, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	
	var data cursorData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("invalid cursor data: %w", err)
	}
	
	return &data, nil
}

// ListRecords lists records with optional filtering and cursor-based pagination
func (p *postgres) ListRecords(ctx context.Context, query model.ListRecordsQuery) (*model.ListRecordsResult, error) {
	// Build the query
	baseQuery := `SELECT id, did, collection, rkey, uri, cid, value, indexed_at, schema_version 
	              FROM records WHERE did = $1`
	args := []interface{}{query.DID}
	argIndex := 2

	// Add collection filter if specified
	if query.Collection != "" {
		baseQuery += fmt.Sprintf(" AND collection = $%d", argIndex)
		args = append(args, query.Collection)
		argIndex++
	}

	// Add time range filters
	if !query.Since.IsZero() {
		baseQuery += fmt.Sprintf(" AND indexed_at >= $%d", argIndex)
		args = append(args, query.Since)
		argIndex++
	}

	if !query.Until.IsZero() {
		baseQuery += fmt.Sprintf(" AND indexed_at <= $%d", argIndex)
		args = append(args, query.Until)
		argIndex++
	}

	// Add cursor condition if provided
	if query.Cursor != "" {
		cursorData, err := decodeCursor(query.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		
		// Add condition to fetch records before the cursor position
		baseQuery += fmt.Sprintf(" AND (indexed_at < $%d OR (indexed_at = $%d AND rkey > $%d))", argIndex, argIndex, argIndex+1)
		args = append(args, cursorData.LastIndexedAt, cursorData.LastRKey)
		argIndex += 2
	}

	// Add ordering and limit
	baseQuery += " ORDER BY indexed_at DESC, rkey ASC"
	
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}
	baseQuery += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit+1) // Fetch one extra record to determine if there are more results

	rows, err := p.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer rows.Close()

	var records []model.Record
	recordCount := 0
	var lastRecord *model.Record
	
	for rows.Next() {
		var record model.Record
		var valueJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.DID,
			&record.Collection,
			&record.RKey,
			&record.URI,
			&record.CID,
			&valueJSON,
			&record.IndexedAt,
			&record.SchemaVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Unmarshal JSON value
		if err := json.Unmarshal(valueJSON, &record.Value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record value: %w", err)
		}

		lastRecord = &record
		recordCount++
		
		// Only add records up to the requested limit
		if recordCount <= limit {
			records = append(records, record)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating records: %w", err)
	}

	result := &model.ListRecordsResult{
		Records: records,
	}
	
	// If we fetched more records than requested, there are more results available
	if recordCount > limit && lastRecord != nil {
		// Generate cursor from the last record we actually returned
		if len(records) > 0 {
			lastReturnedRecord := records[len(records)-1]
			result.NextCursor = encodeCursor(lastReturnedRecord.IndexedAt, lastReturnedRecord.RKey)
		}
	}

	return result, nil
}

// GetRecordByURI retrieves a record by its URI
func (p *postgres) GetRecordByURI(ctx context.Context, uri string) (*model.Record, error) {
	query := `SELECT id, did, collection, rkey, uri, cid, value, indexed_at, schema_version 
	          FROM records WHERE uri = $1`
	
	var record model.Record
	var valueJSON []byte

	err := p.db.QueryRow(ctx, query, uri).Scan(
		&record.ID,
		&record.DID,
		&record.Collection,
		&record.RKey,
		&record.URI,
		&record.CID,
		&valueJSON,
		&record.IndexedAt,
		&record.SchemaVersion,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	// Unmarshal JSON value
	if err := json.Unmarshal(valueJSON, &record.Value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record value: %w", err)
	}

	return &record, nil
}

// CreateMediaAsset creates a new media asset in the database
func (p *postgres) CreateMediaAsset(ctx context.Context, asset model.MediaAsset) error {
	// First check if account exists
	if _, err := p.GetAccount(ctx, asset.DID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("account not found: %s", asset.DID)
		}
		return fmt.Errorf("failed to check account: %w", err)
	}

	query := `INSERT INTO media_assets (asset_id, did, uri, mime_type, size, checksum, created_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`
	
	_, err := p.db.Exec(ctx, query, 
		asset.AssetID, 
		asset.DID, 
		asset.URI, 
		asset.MimeType, 
		asset.Size, 
		asset.Checksum, 
		asset.CreatedAt)
	
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("failed to create media asset: %w", err)
	}
	
	return nil
}

// GetMediaAsset retrieves a media asset by its ID
func (p *postgres) GetMediaAsset(ctx context.Context, assetId string) (*model.MediaAsset, error) {
	query := `SELECT asset_id, did, uri, mime_type, size, checksum, created_at 
	          FROM media_assets WHERE asset_id = $1`
	
	var asset model.MediaAsset
	
	err := p.db.QueryRow(ctx, query, assetId).Scan(
		&asset.AssetID,
		&asset.DID,
		&asset.URI,
		&asset.MimeType,
		&asset.Size,
		&asset.Checksum,
		&asset.CreatedAt,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get media asset: %w", err)
	}
	
	return &asset, nil
}

// UpdateMediaAsset updates an existing media asset
func (p *postgres) UpdateMediaAsset(ctx context.Context, asset model.MediaAsset) error {
	query := `UPDATE media_assets SET did = $1, uri = $2, mime_type = $3, size = $4, checksum = $5, created_at = $6 
	          WHERE asset_id = $7`
	
	result, err := p.db.Exec(ctx, query, 
		asset.DID, 
		asset.URI, 
		asset.MimeType, 
		asset.Size, 
		asset.Checksum, 
		asset.CreatedAt,
		asset.AssetID)
	
	if err != nil {
		return fmt.Errorf("failed to update media asset: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	
	return nil
}

// StoreIdempotentResponse stores an idempotent response in the database
func (p *postgres) StoreIdempotentResponse(ctx context.Context, keyHash, requestHash string, responseBody []byte, statusCode int, expiresAt time.Time) error {
	// First, check if there are existing entries with the same key_hash but different request_hash
	var existingRequestHash string
	query := `SELECT request_hash FROM idempotency WHERE key_hash = $1 AND request_hash != $2 LIMIT 1`
	
	err := p.db.QueryRow(ctx, query, keyHash, requestHash).Scan(&existingRequestHash)
	if err != nil {
		// If no rows found, that's fine - no conflict
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("failed to check for idempotency conflicts: %w", err)
		}
	} else {
		// Found an entry with same key_hash but different request_hash - this is a conflict
		return ErrConflict
	}
	
	// Now try to insert or update
	query = `INSERT INTO idempotency (key_hash, request_hash, response_body, response_status, created_at, expires_at)
	          VALUES ($1, $2, $3, $4, $5, $6)
	          ON CONFLICT (key_hash, request_hash) DO UPDATE 
	          SET response_body = $3, response_status = $4, created_at = $5, expires_at = $6`
	
	_, err = p.db.Exec(ctx, query, keyHash, requestHash, responseBody, statusCode, time.Now().UTC(), expiresAt)
	if err != nil {
		return fmt.Errorf("failed to store idempotent response: %w", err)
	}
	
	return nil
}

// GetIdempotentResponse retrieves a cached idempotent response from the database
func (p *postgres) GetIdempotentResponse(ctx context.Context, keyHash string) ([]byte, int, error) {
	query := `SELECT response_body, response_status FROM idempotency 
	          WHERE key_hash = $1 AND expires_at > $2`
	
	var responseBody []byte
	var statusCode int
	
	err := p.db.QueryRow(ctx, query, keyHash, time.Now().UTC()).Scan(&responseBody, &statusCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("failed to get idempotent response: %w", err)
	}
	
	return responseBody, statusCode, nil
}
