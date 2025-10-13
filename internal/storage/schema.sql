-- Database schema for CDV service

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    did TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Records table
CREATE TABLE IF NOT EXISTS records (
    id TEXT PRIMARY KEY,
    did TEXT NOT NULL REFERENCES accounts(did),
    collection TEXT NOT NULL,
    rkey TEXT NOT NULL,
    uri TEXT NOT NULL UNIQUE,
    cid TEXT NOT NULL,
    value JSONB NOT NULL,
    indexed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    schema_version TEXT NOT NULL,
    UNIQUE(did, collection, rkey)
);

-- Indexes for records table
CREATE INDEX IF NOT EXISTS idx_records_did_collection_indexed_at ON records(did, collection, indexed_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_cid ON records(cid);
CREATE INDEX IF NOT EXISTS idx_records_indexed_at ON records(indexed_at DESC);

-- Media assets table
CREATE TABLE IF NOT EXISTS media_assets (
    asset_id TEXT PRIMARY KEY,
    did TEXT NOT NULL REFERENCES accounts(did),
    uri TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(did, asset_id)
);

-- Operation log table (append-only)
CREATE TABLE IF NOT EXISTS op_log (
    seq BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL,
    ref TEXT NOT NULL,
    did TEXT NOT NULL REFERENCES accounts(did),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for op_log table
CREATE INDEX IF NOT EXISTS idx_op_log_did ON op_log(did);
CREATE INDEX IF NOT EXISTS idx_op_log_type ON op_log(type);
CREATE INDEX IF NOT EXISTS idx_op_log_occurred_at ON op_log(occurred_at);
