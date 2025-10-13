#!/bin/bash

# seed.sh - Deterministic seed script for CDV service
# This script creates a consistent test dataset for development and testing

set -e

# Default configuration
CDV_URL=${CDV_URL:-"http://localhost:8082"}
IDENTITY_URL=${IDENTITY_URL:-"http://localhost:8081"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if required tools are available
tool_check() {
  if ! command -v $1 &> /dev/null; then
    error "$1 is required but not installed"
    exit 1
  fi
}

tool_check curl
tool_check jq

echo "========================================="
echo "RegistryAccord CDV Seeding Script"
echo "========================================="

# Create test users
log "Creating test users..."

# In a real implementation, this would interact with the identity service
# For now, we'll just log that this would happen
echo "Test users would be created via the Identity service at $IDENTITY_URL"

# Create sample records
log "Creating sample records..."

# Sample post record
cat > /tmp/sample_post.json << EOF
{
  "did": "did:ra:test123",
  "collection": "com.registryaccord.feed.post",
  "record": {
    "text": "Hello, RegistryAccord! This is a test post.",
    "createdAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "authorDid": "did:ra:test123"
  }
}
EOF

echo "Sample record created at /tmp/sample_post.json"

# Sample profile record
cat > /tmp/sample_profile.json << EOF
{
  "did": "did:ra:test123",
  "collection": "com.registryaccord.profile",
  "record": {
    "displayName": "Test User",
    "bio": "This is a test user for RegistryAccord CDV development."
  }
}
EOF

echo "Sample profile created at /tmp/sample_profile.json"

log "Seed data preparation complete!"
log "To use this data, you would POST these records to the CDV service"

# Cleanup
cleanup() {
  rm -f /tmp/sample_post.json /tmp/sample_profile.json
}

trap cleanup EXIT

log "Script completed successfully!"
