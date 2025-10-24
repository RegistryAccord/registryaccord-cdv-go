// internal/server/mux.go
// Package server implements the HTTP handlers and routing for the CDV service.
// It provides RESTful endpoints for record and media operations with JWT authentication,
// schema validation, and event publishing.
package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	errordefs "github.com/RegistryAccord/registryaccord-cdv-go/internal/errors"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/event"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/identity"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/jwks"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/media"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/metrics"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/model"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/schema"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/storage"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ContextKey is used for context values to avoid collisions
// when storing values in request context
type ContextKey string

const (
	// Context keys for storing request-scoped values
	ContextKeyDID ContextKey = "did"           // Stores the DID from JWT
	ContextKeyCorrelationID ContextKey = "correlationId" // Unique ID for request tracking

	// Default limits for list operations
	DefaultListLimit = 25  // Default number of records to return
	MaxListLimit = 100     // Maximum number of records to return
)

// Mux handles HTTP requests for the CDV service.
// It implements all the required endpoints and manages dependencies
// such as storage, event publishing, and identity validation.
type Mux struct {
	mux *http.ServeMux          // HTTP request multiplexer
	s   storage.Store           // Storage interface for records and media
	p   event.Publisher         // Event publisher for streaming updates
	id  *identity.Client        // Identity client for DID validation
	jwksClient *jwks.Client     // JWKS client for JWT validation
	jwtIssuer string           // Expected JWT issuer for validation
	jwtAudience string         // Expected JWT audience for validation
	validator *schema.Validator // Schema validator for record validation
	mediaClient *media.S3Client // S3 client for media storage operations
	metrics     *metrics.Metrics // Metrics for monitoring
	
	// Media limits
	maxMediaSize int64      // Maximum media size in bytes
	allowedMimeTypes []string // Allowed MIME types for media uploads
	
	// Schema policy
	rejectDeprecatedSchemas bool // Whether to reject deprecated schemas
	
	// CORS configuration
	corsAllowedOrigins []string // Allowed origins for CORS (empty means deny all)
}

// NewMux creates a new HTTP mux with all CDV endpoints.
// It initializes all dependencies and registers the HTTP handlers.
// Parameters:
//   - s: Storage interface for data persistence
//   - p: Event publisher for streaming updates
//   - id: Identity client for DID validation (can be nil)
//   - jwtIssuer: Expected JWT issuer for validation
//   - jwtAudience: Expected JWT audience for validation
//   - specsURL: URL to the specs repository for schema resolution
//   - rejectDeprecatedSchemas: Whether to reject deprecated schemas
func NewMux(s storage.Store, p event.Publisher, id *identity.Client, jwtIssuer, jwtAudience string, maxMediaSize int64, allowedMimeTypes []string, jwksClient *jwks.Client, specsURL string, rejectDeprecatedSchemas bool) *http.ServeMux {
	// Initialize schema validator
	validator, err := schema.NewValidator()
	if err != nil {
		slog.Error("failed to initialize schema validator", "error", err)
		os.Exit(1)
	}

	// Initialize media client if S3 configuration is present
	var mediaClient *media.S3Client
	if os.Getenv("CDV_S3_ENDPOINT") != "" && os.Getenv("CDV_S3_BUCKET") != "" {
		mediaClient, err = media.NewS3Client(
			os.Getenv("CDV_S3_ENDPOINT"),
			os.Getenv("CDV_S3_REGION"),
			os.Getenv("CDV_S3_ACCESS_KEY_ID"),
			os.Getenv("CDV_S3_SECRET_ACCESS_KEY"),
			os.Getenv("CDV_S3_BUCKET"),
		)
		if err != nil {
			slog.Error("failed to initialize S3 client", "error", err)
			os.Exit(1)
		}
	}

	// Use provided JWKS client or create a new one
	if jwksClient == nil {
		jwksClient = jwks.NewClient(fmt.Sprintf("%s/.well-known/jwks.json", jwtIssuer))
	}
	
	// Update validator with the specs URL
	resolver := schema.NewResolver(specsURL, "/tmp/registryaccord-specs-cache")
	validator.SetResolver(resolver)

	m := &Mux{
		mux:         http.NewServeMux(),
		s:           s,
		p:           p,
		id:          id,
		jwksClient:  jwksClient,
		jwtIssuer:   jwtIssuer,
		jwtAudience: jwtAudience,
		validator:   validator,
		mediaClient: mediaClient,
		metrics:     metrics.NewMetrics(),
		maxMediaSize: maxMediaSize,
		allowedMimeTypes: allowedMimeTypes,
		rejectDeprecatedSchemas: rejectDeprecatedSchemas,
	}

	// Register health endpoints
	m.mux.HandleFunc("/healthz", m.handleHealthz)
	m.mux.HandleFunc("/readyz", m.handleReadyz)
	m.mux.Handle("/metrics", promhttp.Handler())

	// Register Phase 1 CDV endpoints with appropriate middleware
	m.mux.HandleFunc("/v1/repo/record", m.method("POST", m.withMiddleware(m.handleCreateRecord)))
	m.mux.HandleFunc("/v1/repo/listRecords", m.method("GET", m.withMiddleware(m.handleListRecords)))
	m.mux.HandleFunc("/v1/media/uploadInit", m.method("POST", m.withMiddleware(m.handleUploadInit)))
	m.mux.HandleFunc("/v1/media/finalize", m.method("POST", m.withMiddleware(m.handleFinalize)))
	m.mux.HandleFunc("/v1/media/", m.method("GET", m.withMiddleware(m.handleGetMediaMeta)))

	return m.mux
}

// method ensures the HTTP method matches the expected method
func (m *Mux) method(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			err := errordefs.New(errordefs.CDV_BAD_REQUEST, "method not allowed", "")
			m.writeErrorDef(w, err)
			return
		}
		h(w, r)
	}
}

// withMiddleware applies common middleware to handlers
func (m *Mux) withMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Handle CORS preflight requests
		if r.Method == "OPTIONS" {
			// Set CORS headers
			if len(m.corsAllowedOrigins) > 0 {
				origin := r.Header.Get("Origin")
				if origin != "" {
					// Check if origin is allowed
					allowed := false
					for _, allowedOrigin := range m.corsAllowedOrigins {
						if allowedOrigin == "*" || allowedOrigin == origin {
							allowed = true
							break
						}
					}
					if allowed {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Correlation-Id")
						w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// Set CORS headers for regular requests
		if len(m.corsAllowedOrigins) > 0 {
			origin := r.Header.Get("Origin")
			if origin != "" {
				// Check if origin is allowed
				allowed := false
				for _, allowedOrigin := range m.corsAllowedOrigins {
					if allowedOrigin == "*" || allowedOrigin == origin {
						allowed = true
						break
					}
				}
				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
		}

		// Add correlation ID if not present
		correlationID := r.Header.Get("X-Correlation-Id")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		r = r.WithContext(context.WithValue(r.Context(), ContextKeyCorrelationID, correlationID))
		w.Header().Set("X-Correlation-Id", correlationID)

		// Apply JWT authentication for mutating endpoints
		if r.Method == "POST" || strings.HasPrefix(r.URL.Path, "/v1/media/") {
			did, err := m.validateJWT(r)
			if err != nil {
				// Check if err is already an errordefs.Error or create a new one
				var errorDef *errordefs.Error
				if e, ok := err.(*errordefs.Error); ok {
					errorDef = e
					errorDef.CorrelationID = correlationID
				} else {
					errorDef = errordefs.New(errordefs.CDV_AUTHZ, err.Error(), correlationID)
				}
				m.writeErrorDef(w, errorDef)
				m.logRequest(r, errorDef.HTTPStatus, time.Since(start), correlationID, err)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ContextKeyDID, did))
		}

		// Call the handler
		h(w, r)
	}
}

// validateJWT validates a JWT and extracts the DID using JWKS
func (m *Mux) validateJWT(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errordefs.New(errordefs.CDV_AUTHN, "missing Authorization header", "")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", errordefs.New(errordefs.CDV_AUTHN, "invalid Authorization header format", "")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate JWT using JWKS
	claims, err := m.jwksClient.ValidateJWT(r.Context(), tokenString, m.jwtIssuer, m.jwtAudience)
	if err != nil {
		// Map specific JWT validation errors to appropriate error codes
		errStr := err.Error()
		if strings.Contains(errStr, "expired") {
			return "", errordefs.New(errordefs.CDV_JWT_EXPIRED, "JWT token expired", "")
		} else if strings.Contains(errStr, "invalid issuer") {
			return "", errordefs.New(errordefs.CDV_JWT_INVALID, "invalid JWT issuer", "")
		} else if strings.Contains(errStr, "invalid audience") {
			return "", errordefs.New(errordefs.CDV_JWT_INVALID, "invalid JWT audience", "")
		} else if strings.Contains(errStr, "kid") {
			return "", errordefs.New(errordefs.CDV_JWT_MALFORMED, "missing or invalid kid in JWT header", "")
		} else if strings.Contains(errStr, "key") {
			return "", errordefs.New(errordefs.CDV_JWT_INVALID, "failed to get key for JWT validation", "")
		} else if strings.Contains(errStr, "signature") || strings.Contains(errStr, "verify") {
			return "", errordefs.New(errordefs.CDV_JWT_INVALID, "invalid JWT signature", "")
		} else {
			return "", errordefs.New(errordefs.CDV_JWT_INVALID, fmt.Sprintf("failed to validate JWT: %v", err), "")
		}
	}

	did, ok := claims["sub"].(string)
	if !ok || did == "" {
		return "", errordefs.New(errordefs.CDV_JWT_INVALID, "missing or invalid sub claim", "")
	}

	return did, nil
}

// writeSuccess writes a successful response
func (m *Mux) writeSuccess(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"data": data,
	}
	_ = json.NewEncoder(w).Encode(response)
}

// writeError writes an error response following the CDV error taxonomy
func (m *Mux) writeError(w http.ResponseWriter, statusCode int, code, message, correlationID string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":          code,
			"message":       message,
			"correlationId": correlationID,
		},
	}
	
	if details != nil {
		response["error"].(map[string]interface{})["details"] = details
	}
	
	_ = json.NewEncoder(w).Encode(response)
}

// writeErrorDef writes an error response using the error definitions package
func (m *Mux) writeErrorDef(w http.ResponseWriter, err *errordefs.Error) {
	m.writeError(w, err.HTTPStatus, string(err.Code), err.Message, err.CorrelationID, err.Details)
}

// logRequest logs request details
func (m *Mux) logRequest(r *http.Request, status int, duration time.Duration, correlationID string, err error) {
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.Duration("duration", duration),
		slog.String("user_agent", r.UserAgent()),
		slog.String("remote_addr", r.RemoteAddr),
	}
	
	if correlationID != "" {
		attrs = append(attrs, slog.String("correlation_id", correlationID))
	}
	
	if did, ok := r.Context().Value(ContextKeyDID).(string); ok && did != "" {
		attrs = append(attrs, slog.String("did", did))
	}
	
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		slog.LogAttrs(r.Context(), slog.LevelError, "request completed with error", attrs...)
	} else {
		slog.LogAttrs(r.Context(), slog.LevelInfo, "request completed", attrs...)
	}
}

// handleHealthz handles liveness health check requests
func (m *Mux) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleReadyz handles readiness health check requests
func (m *Mux) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// Check if the service is ready to serve requests
	// This should check dependencies like database connectivity
	
	// For now, we'll do a simple database check
	// In a real implementation, you might check more dependencies
	
	// Test database connectivity by doing a simple query
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	// Try to get a non-existent account to test database connectivity
	_, err := m.s.GetAccount(ctx, "health-check")
	
	// We expect ErrNotFound, which means the database is accessible
	// Any other error indicates a problem
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
		return
	}
	
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleCreateRecord handles POST /v1/repo/record with idempotency support
func (m *Mux) handleCreateRecord(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cdv-service").Start(r.Context(), "handleCreateRecord")
	defer span.End()
	defer r.Body.Close()
	
	var req model.CreateRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "invalid JSON")
		err := errordefs.New(errordefs.CDV_VALIDATION, "invalid JSON", correlationID)
		m.writeErrorDef(w, err)
		return
	}
	
	// Add request attributes to span
	span.SetAttributes(
		attribute.String("collection", req.Collection),
		attribute.String("did", req.DID),
		attribute.Bool("has_record", req.Record != nil),
		attribute.Bool("has_idempotency_key", req.IdempotencyKey != ""),
	)

	// Validate required fields
	if req.Collection == "" || req.DID == "" || req.Record == nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_VALIDATION, "collection, did, and record are required", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Validate DID matches JWT subject (Phase 1 requirement)
	jwtDID := ctx.Value(ContextKeyDID).(string)
	if req.DID != jwtDID {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_DID_MISMATCH, "DID must match JWT subject", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Check for idempotency key
	if req.IdempotencyKey != "" {
		// Hash the idempotency key
		keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.IdempotencyKey)))
		
		// Try to get cached response
		if responseBody, statusCode, err := m.s.GetIdempotentResponse(ctx, keyHash); err == nil {
			// Return cached response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write(responseBody)
			return
		}
	}

	// Validate record against schema
	schemaVersion, err := m.validator.Validate(req.Collection, req.Record)
	if err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.NewWithDetails(errordefs.CDV_SCHEMA_REJECT, fmt.Sprintf("schema validation failed: %v", err), correlationID, err.Error())
		m.writeErrorDef(w, err)
		return
	}
	
	// Resolve the latest schema version for this collection
	resolvedVersion, err := m.validator.ResolveSchemaVersion(req.Collection)
	if err != nil {
		slog.Warn("failed to resolve schema version, using validated version", "collection", req.Collection, "error", err)
	} else {
		// Check if the resolved version is deprecated
		if strings.HasSuffix(resolvedVersion, ":deprecated") {
			// Remove the deprecated suffix for storage
			actualVersion := strings.TrimSuffix(resolvedVersion, ":deprecated")
			
			// Log a warning about using a deprecated schema
			slog.Warn("using deprecated schema version", "collection", req.Collection, "version", actualVersion)
			
			// In a production environment, you might want to reject deprecated schemas
			// after a certain date, but for now we'll accept them with a warning
			schemaVersion = actualVersion
		} else {
			// Use the resolved version if available
			schemaVersion = resolvedVersion
		}
	}

	// Create account if it doesn't exist
	if _, err := m.s.GetAccount(ctx, req.DID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			if err := m.s.CreateAccount(ctx, req.DID); err != nil {
				correlationID := ctx.Value(ContextKeyCorrelationID).(string)
				err := errordefs.New(errordefs.CDV_INTERNAL, "failed to create account", correlationID)
				m.writeErrorDef(w, err)
				return
			}
		} else {
			correlationID := ctx.Value(ContextKeyCorrelationID).(string)
			err := errordefs.New(errordefs.CDV_INTERNAL, "failed to check account", correlationID)
			m.writeErrorDef(w, err)
			return
		}
	}

	// Generate record ID and URI
	recordID := uuid.New().String()
	// Generate ULID for RKey to ensure lexicographical ordering and collision resistance
	entropy := ulid.Monotonic(rand.Reader, 0)
	rKey := ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
	uri := fmt.Sprintf("at://%s/%s/%s", req.DID, req.Collection, rKey)
	cid := uuid.New().String() // In a real implementation, this would be a content hash

	// Use provided createdAt or current time
	var indexedAt time.Time
	if req.CreatedAt != nil {
		indexedAt = *req.CreatedAt
	} else {
		indexedAt = time.Now().UTC()
	}

	// Create the record
	record := model.Record{
		ID:           recordID,
		DID:          req.DID,
		Collection:   req.Collection,
		RKey:         rKey,
		URI:          uri,
		CID:          cid,
		Value:        req.Record,
		IndexedAt:    indexedAt,
		SchemaVersion: schemaVersion, // Use the schema version from validation
	}

	start := time.Now()
	if err := m.s.CreateRecord(ctx, record); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		if errors.Is(err, storage.ErrConflict) {
			err := errordefs.New(errordefs.CDV_CONFLICT, "record already exists", correlationID)
			m.writeErrorDef(w, err)
			m.logRequest(r, http.StatusConflict, time.Since(start), correlationID, err)
			return
		}
		err := errordefs.New(errordefs.CDV_INTERNAL, "failed to create record", correlationID)
		m.writeErrorDef(w, err)
		m.logRequest(r, http.StatusInternalServerError, time.Since(start), correlationID, err)
		return
	}

	// Publish record created event
	if err := m.p.PublishRecordCreated(ctx, req.Collection, record); err != nil {
		slog.Warn("failed to publish record created event", "error", err)
	}

	response := model.CreateRecordData{
		URI:       uri,
		CID:       cid,
		IndexedAt: indexedAt,
	}

	// Store response for idempotency if key was provided
	if req.IdempotencyKey != "" {
		keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.IdempotencyKey)))
		// Calculate request hash for conflict detection
		requestBytes, _ := json.Marshal(req)
		requestHash := fmt.Sprintf("%x", sha256.Sum256(requestBytes))
		responseBody, _ := json.Marshal(map[string]interface{}{"data": response})
		expiresAt := time.Now().UTC().Add(24 * time.Hour) // 24-hour expiration
		
		// Try to store the idempotent response
		// If there's a conflict with a different request hash, this should return an error
		if err := m.s.StoreIdempotentResponse(ctx, keyHash, requestHash, responseBody, http.StatusOK, expiresAt); err != nil {
			// Check if this is a conflict error (different payload for same idempotency key)
			if errors.Is(err, storage.ErrConflict) {
				correlationID := ctx.Value(ContextKeyCorrelationID).(string)
				err := errordefs.New(errordefs.CDV_CONFLICT, "idempotency key conflict: different payload for same key", correlationID)
				m.writeErrorDef(w, err)
				return
			}
			// For other errors, log and continue (don't fail the request for idempotency issues)
			slog.Warn("failed to store idempotent response", "error", err)
		}
	}

	m.writeSuccess(w, http.StatusOK, response)
	m.logRequest(r, http.StatusOK, time.Since(start), ctx.Value(ContextKeyCorrelationID).(string), nil)
}

// handleListRecords handles GET /v1/repo/listRecords
func (m *Mux) handleListRecords(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cdv-service").Start(r.Context(), "handleListRecords")
	defer span.End()
	
	start := time.Now()
	did := r.URL.Query().Get("did")
	if did == "" {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "did is required")
		err := errordefs.New(errordefs.CDV_VALIDATION, "did is required", correlationID)
		m.writeErrorDef(w, err)
		m.logRequest(r, http.StatusBadRequest, time.Since(start), correlationID, errors.New("did is required"))
		return
	}
	
	// Add request attributes to span
	span.SetAttributes(
		attribute.String("did", did),
	)

	collection := r.URL.Query().Get("collection")
	
	// Add more request attributes to span
	span.SetAttributes(
		attribute.String("collection", collection),
	)

	limit := DefaultListLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			if v > 0 && v <= MaxListLimit {
				limit = v
			} else if v > MaxListLimit {
				limit = MaxListLimit
			}
		}
	}

	// Parse time filters
	var since, until time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
			span.SetAttributes(attribute.String("since", sinceStr))
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = t
			span.SetAttributes(attribute.String("until", untilStr))
		}
	}

	query := model.ListRecordsQuery{
		DID:        did,
		Collection: collection,
		Limit:      limit,
		Cursor:     r.URL.Query().Get("cursor"),
		Since:      since,
		Until:      until,
	}

	result, err := m.s.ListRecords(ctx, query)
	if err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "failed to list records")
		
		// Check if this is a cursor validation error
		if strings.Contains(err.Error(), "invalid cursor") {
			err := errordefs.New(errordefs.CDV_CURSOR_INVALID, err.Error(), correlationID)
			m.writeErrorDef(w, err)
			return
		}
		
		// For all other errors, return internal error
		errDef := errordefs.New(errordefs.CDV_INTERNAL, "failed to list records", correlationID)
		m.writeErrorDef(w, errDef)
		return
	}

	m.writeSuccess(w, http.StatusOK, result)
}

// handleUploadInit handles POST /v1/media/uploadInit
func (m *Mux) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cdv-service").Start(r.Context(), "handleUploadInit")
	defer span.End()
	defer r.Body.Close()
	
	var req model.UploadInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "invalid JSON")
		err := errordefs.New(errordefs.CDV_VALIDATION, "invalid JSON", correlationID)
		m.writeErrorDef(w, err)
		return
	}
	
	// Add request attributes to span
	span.SetAttributes(
		attribute.String("did", req.DID),
		attribute.String("mimeType", req.MimeType),
		attribute.Int64("size", req.Size),
		attribute.Bool("has_filename", req.Filename != ""),
	)

	// Validate required fields
	if req.DID == "" || req.MimeType == "" || req.Size <= 0 {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_VALIDATION, "did, mimeType, and size are required", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Validate media size limit
	if req.Size > m.maxMediaSize {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_MEDIA_SIZE, fmt.Sprintf("media size exceeds limit of %d bytes", m.maxMediaSize), correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Validate media type
	allowed := false
	for _, mimeType := range m.allowedMimeTypes {
		if req.MimeType == mimeType {
			allowed = true
			break
		}
	}
	if !allowed {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_MEDIA_TYPE, fmt.Sprintf("media type %s is not allowed", req.MimeType), correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Validate DID matches JWT subject (Phase 1 requirement)
	jwtDID := ctx.Value(ContextKeyDID).(string)
	if req.DID != jwtDID {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_DID_MISMATCH, "DID must match JWT subject", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Create account if it doesn't exist
	if _, err := m.s.GetAccount(ctx, req.DID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			if err := m.s.CreateAccount(ctx, req.DID); err != nil {
				correlationID := ctx.Value(ContextKeyCorrelationID).(string)
				err := errordefs.New(errordefs.CDV_INTERNAL, "failed to create account", correlationID)
				m.writeErrorDef(w, err)
				return
			}
		} else {
			correlationID := ctx.Value(ContextKeyCorrelationID).(string)
			err := errordefs.New(errordefs.CDV_INTERNAL, "failed to check account", correlationID)
			m.writeErrorDef(w, err)
			return
		}
	}

	// Generate asset ID
	assetID := uuid.New().String()
	uri := fmt.Sprintf("at://%s/media/%s", req.DID, assetID)

	// Create the media asset record
	asset := model.MediaAsset{
		AssetID:   assetID,
		DID:       req.DID,
		URI:       uri,
		MimeType:  req.MimeType,
		Size:      req.Size,
		Checksum:  req.SHA256,
		CreatedAt: time.Now().UTC(),
	}

	if err := m.s.CreateMediaAsset(ctx, asset); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		if errors.Is(err, storage.ErrConflict) {
			err := errordefs.New(errordefs.CDV_CONFLICT, "asset already exists", correlationID)
			m.writeErrorDef(w, err)
			return
		}
		err := errordefs.New(errordefs.CDV_INTERNAL, "failed to create media asset", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Generate object key
	objectKey := fmt.Sprintf("%s/%s/%s", os.Getenv("CDV_ENV"), req.DID, assetID)
	if req.Filename != "" {
		objectKey += "/" + req.Filename
	}

	// Generate presigned URL for S3 upload
	var uploadURL string
	var expiresAt time.Time
	if m.mediaClient != nil {
		expiresAt = time.Now().Add(15 * time.Minute)
		var err error
		uploadURL, err = m.mediaClient.GenerateUploadURL(ctx, objectKey, 15*time.Minute)
		if err != nil {
			correlationID := ctx.Value(ContextKeyCorrelationID).(string)
			err := errordefs.New(errordefs.CDV_INTERNAL, "failed to generate upload URL", correlationID)
			m.writeErrorDef(w, err)
			return
		}
	} else {
		// Fallback to simplified implementation if S3 is not configured
		uploadURL = fmt.Sprintf("http://localhost:8081/upload/%s", assetID)
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	// Store the object key in the asset metadata
	asset.URI = fmt.Sprintf("s3://%s/%s", os.Getenv("CDV_S3_BUCKET"), objectKey)

	response := model.UploadInitData{
		AssetID:   assetID,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt,
	}

	m.writeSuccess(w, http.StatusOK, response)
}

// handleFinalize handles POST /v1/media/finalize
func (m *Mux) handleFinalize(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cdv-service").Start(r.Context(), "handleFinalize")
	defer span.End()
	defer r.Body.Close()
	
	var req model.FinalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "invalid JSON")
		err := errordefs.New(errordefs.CDV_VALIDATION, "invalid JSON", correlationID)
		m.writeErrorDef(w, err)
		return
	}
	
	// Add request attributes to span
	span.SetAttributes(
		attribute.String("assetId", req.AssetID),
		attribute.String("sha256", req.SHA256),
	)

	// Validate required fields
	if req.AssetID == "" || req.SHA256 == "" {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_VALIDATION, "assetId and sha256 are required", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Get the media asset
	asset, err := m.s.GetMediaAsset(ctx, req.AssetID)
	if err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		if errors.Is(err, storage.ErrNotFound) {
			err := errordefs.New(errordefs.CDV_NOT_FOUND, "asset not found", correlationID)
			m.writeErrorDef(w, err)
			return
		}
		err := errordefs.New(errordefs.CDV_INTERNAL, "failed to get media asset", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Validate DID matches JWT subject (Phase 1 requirement)
	jwtDID := ctx.Value(ContextKeyDID).(string)
	if asset.DID != jwtDID {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_DID_MISMATCH, "DID must match JWT subject", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Verify object exists and checksum matches if S3 is configured
	if m.mediaClient != nil {
		// Extract object key from URI
		objectKey := strings.TrimPrefix(asset.URI, fmt.Sprintf("s3://%s/", os.Getenv("CDV_S3_BUCKET")))
		
		valid, size, err := m.mediaClient.VerifyObject(ctx, objectKey, req.SHA256)
		if err != nil {
			correlationID := ctx.Value(ContextKeyCorrelationID).(string)
			err := errordefs.New(errordefs.CDV_INTERNAL, "failed to verify media object", correlationID)
			m.writeErrorDef(w, err)
			return
		}
		
		if !valid {
			correlationID := ctx.Value(ContextKeyCorrelationID).(string)
			err := errordefs.New(errordefs.CDV_MEDIA_CHECKSUM, "checksum verification failed", correlationID)
			m.writeErrorDef(w, err)
			return
		}
		
		// Update asset size if it was verified
		asset.Size = size
	}

	// Update the asset with the checksum
	asset.Checksum = req.SHA256
	if err := m.s.UpdateMediaAsset(ctx, *asset); err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		err := errordefs.New(errordefs.CDV_INTERNAL, "failed to update media asset", correlationID)
		m.writeErrorDef(w, err)
		return
	}

	// Publish media finalized event
	if err := m.p.PublishMediaFinalized(ctx, *asset); err != nil {
		slog.Warn("failed to publish media finalized event", "error", err)
	}

	m.writeSuccess(w, http.StatusOK, asset)
}

// handleGetMediaMeta handles GET /v1/media/:assetId/meta
func (m *Mux) handleGetMediaMeta(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cdv-service").Start(r.Context(), "handleGetMediaMeta")
	defer span.End()
	
	// Extract assetId from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/media/")
	assetID := strings.TrimSuffix(path, "/meta")

	if assetID == "" {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		span.SetStatus(codes.Error, "assetId is required")
		m.writeError(w, http.StatusBadRequest, "CDV_VALIDATION", "assetId is required", correlationID, nil)
		return
	}
	
	// Add request attributes to span
	span.SetAttributes(
		attribute.String("assetId", assetID),
	)

	// Get the media asset
	asset, err := m.s.GetMediaAsset(ctx, assetID)
	if err != nil {
		correlationID := ctx.Value(ContextKeyCorrelationID).(string)
		if errors.Is(err, storage.ErrNotFound) {
			m.writeError(w, http.StatusNotFound, "CDV_NOT_FOUND", "asset not found", correlationID, nil)
			return
		}
		m.writeError(w, http.StatusInternalServerError, "CDV_INTERNAL", "failed to get media asset", correlationID, nil)
		return
	}

	m.writeSuccess(w, http.StatusOK, asset)
}
