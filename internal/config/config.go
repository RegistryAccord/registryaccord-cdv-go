// Package config provides configuration loading and management for the CDV service.
// It handles environment variable parsing and provides default values for all settings.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// init loads environment variables from .env files during package initialization.
// In development, it loads .env and .env.local files if they exist.
// In production, it relies solely on system environment variables.
// The loading order ensures that system environment variables take precedence over .env files.
func init() {
	// In dev, load .env files if they exist; in production, rely only on environment variables
	// godotenv.Load() does not override already-set environment variables,
	// preserving OS env > .env precedence

	// Load .env file if it exists (for shared development config)
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load .env file: %v\n", err)
		}
	}

	// Load .env.local if it exists (for local overrides, gitignored)
	if _, err := os.Stat(".env.local"); err == nil {
		if err := godotenv.Load(".env.local"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load .env.local file: %v\n", err)
		}
	}
}

// Config captures environment-driven settings for the CDV service.
// It contains all configuration parameters needed to run the CDV service.
type Config struct {
	Env          string // Deployment environment (dev, staging, prod)
	Port         string // HTTP server port
	DatabaseDSN  string // Database connection string (PostgreSQL)
	NATSURL      string // NATS server URL
	S3Endpoint   string // S3-compatible storage endpoint
	S3Region     string // S3 region
	S3Bucket     string // S3 bucket name
	S3AccessKey  string // S3 access key
	S3SecretKey  string // S3 secret key
	JWTIssuer    string // Expected issuer for JWT validation
	JWTAudience  string // Expected audience for JWT validation
	IdentityURL  string // Identity service URL for DID validation
	SpecsURL     string // URL to the specs repository for schema resolution
	
	// Media limits
	MaxMediaSize int64    // Maximum media size in bytes (default 10MB)
	AllowedMimeTypes []string // Allowed MIME types for media uploads
	
	// Schema policy
	RejectDeprecatedSchemas bool // Whether to reject deprecated schemas
	
	// CORS configuration
	CORSAllowedOrigins []string // Allowed origins for CORS (empty means deny all)
}

// Default configuration values used when environment variables are not set
const (
	defaultPort       = "8080"              // Default HTTP server port
	defaultS3Region   = "us-east-1"         // Default S3 region
	defaultEnv        = "dev"               // Default environment
)

// Load reads environment variables and produces a Config suitable for wiring the service.
// It handles both required and optional configuration parameters, providing defaults where appropriate.
// Returns an error if required parameters are missing or invalid.
func Load() (Config, error) {
	cfg := Config{}

	// Handle environment variable
	if env, exists := os.LookupEnv("CDV_ENV"); exists {
		cfg.Env = env
	} else {
		cfg.Env = defaultEnv
	}

	// Handle port
	if port, exists := os.LookupEnv("CDV_PORT"); exists {
		cfg.Port = port
	} else {
		cfg.Port = defaultPort
	}

	// Handle optional variables
	if dsn, exists := os.LookupEnv("CDV_DB_DSN"); exists {
		cfg.DatabaseDSN = dsn
	}

	if natsURL, exists := os.LookupEnv("CDV_NATS_URL"); exists {
		cfg.NATSURL = natsURL
	}

	if s3Endpoint, exists := os.LookupEnv("CDV_S3_ENDPOINT"); exists {
		cfg.S3Endpoint = s3Endpoint
	}

	if s3Region, exists := os.LookupEnv("CDV_S3_REGION"); exists {
		cfg.S3Region = s3Region
	} else {
		cfg.S3Region = defaultS3Region
	}

	if s3Bucket, exists := os.LookupEnv("CDV_S3_BUCKET"); exists {
		cfg.S3Bucket = s3Bucket
	}

	if s3AccessKey, exists := os.LookupEnv("CDV_S3_ACCESS_KEY"); exists {
		cfg.S3AccessKey = s3AccessKey
	}

	if s3SecretKey, exists := os.LookupEnv("CDV_S3_SECRET_KEY"); exists {
		cfg.S3SecretKey = s3SecretKey
	}

	if jwtIssuer, exists := os.LookupEnv("CDV_JWT_ISSUER"); exists {
		cfg.JWTIssuer = jwtIssuer
	}

	if jwtAudience, exists := os.LookupEnv("CDV_JWT_AUDIENCE"); exists {
		cfg.JWTAudience = jwtAudience
	}

	if identityURL, exists := os.LookupEnv("IDENTITY_URL"); exists {
		cfg.IdentityURL = identityURL
	}
	
	if specsURL, exists := os.LookupEnv("CDV_SPECS_URL"); exists {
		cfg.SpecsURL = specsURL
	} else {
		cfg.SpecsURL = "https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas"
	}
	
	// Handle media limits
	if maxMediaSize, exists := os.LookupEnv("CDV_MAX_MEDIA_SIZE"); exists {
		if size, err := strconv.ParseInt(maxMediaSize, 10, 64); err == nil {
			cfg.MaxMediaSize = size
		}
	} else {
		// Default to 10MB
		cfg.MaxMediaSize = 10 * 1024 * 1024
	}
	
	if allowedMimeTypes, exists := os.LookupEnv("CDV_ALLOWED_MIME_TYPES"); exists {
		cfg.AllowedMimeTypes = strings.Split(allowedMimeTypes, ",")
		// Trim whitespace from each MIME type
		for i, mimeType := range cfg.AllowedMimeTypes {
			cfg.AllowedMimeTypes[i] = strings.TrimSpace(mimeType)
		}
	} else {
		// Default allowed MIME types
		cfg.AllowedMimeTypes = []string{"image/jpeg", "image/png", "image/gif", "video/mp4"}
	}
	
	// Handle deprecation policy
	if rejectDeprecated, exists := os.LookupEnv("CDV_REJECT_DEPRECATED_SCHEMAS"); exists {
		cfg.RejectDeprecatedSchemas = parseBool(rejectDeprecated)
	}
	
	// Handle CORS configuration
	if corsOrigins, exists := os.LookupEnv("CDV_CORS_ALLOWED_ORIGINS"); exists {
		cfg.CORSAllowedOrigins = strings.Split(corsOrigins, ",")
		// Trim whitespace from each origin
		for i, origin := range cfg.CORSAllowedOrigins {
			cfg.CORSAllowedOrigins[i] = strings.TrimSpace(origin)
		}
	}

	// Validate required parameters
	if cfg.JWTIssuer == "" {
		return cfg, fmt.Errorf("CDV_JWT_ISSUER is required")
	}
	
	if cfg.JWTAudience == "" {
		return cfg, fmt.Errorf("CDV_JWT_AUDIENCE is required")
	}
	
	return cfg, nil
}

// getEnv retrieves an environment variable value, returning a fallback if not set or empty
func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists && v != "" {
		return v
	}
	return fallback
}

// parseBool converts a string to a boolean value, returning false if parsing fails
func parseBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
