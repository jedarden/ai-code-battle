package main

import (
	"database/sql"
	"testing"
)

// TestNewEnrichmentService_B2Only tests the R2→B2 fallback logic
func TestNewEnrichmentService_B2Only(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		PostgresHost:          "localhost",
		PostgresPort:          5432,
		PostgresUser:          "test",
		PostgresPassword:      "test",
		PostgresDatabase:      "test",
		DatabaseName:          "test",
		LLMBaseURL:            "https://api.test.com",
		LLMAPIKey:             "test-key",
		LLMModel:              "test-model",
		// R2 not configured - should fall back to B2
		R2AccessKeyID:     "",
		R2SecretAccessKey: "",
		R2Endpoint:        "",
		R2BucketName:      "",
		// B2 configured
		B2AccessKeyID:     "b2-key",
		B2SecretAccessKey: "b2-secret",
		B2Endpoint:        "https://b2.test.com",
		B2BucketName:      "test-bucket",
		MinTurnCount:          100,
		MinWinProbCrossings:   3,
		UpsetThreshold:        150.0,
		MaxEnrichmentsPerHour: 20,
		MaxConcurrentRequests: 3,
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.store == nil {
		t.Error("Expected store to be initialized")
	}
	if svc.selector == nil {
		t.Error("Expected selector to be initialized")
	}
	if svc.generator == nil {
		t.Error("Expected generator to be initialized")
	}
	if svc.r2Client == nil {
		t.Error("Expected r2Client to be initialized (even without credentials)")
	}
	if svc.b2Client == nil {
		t.Error("Expected b2Client to be initialized")
	}
	if svc.llmClient == nil {
		t.Error("Expected llmClient to be initialized")
	}

	// Verify that B2 client is used (not R2) when R2 has no credentials
	if svc.r2Client.HasCredentials() {
		t.Error("Expected R2 client to have no credentials")
	}
	if !svc.b2Client.HasCredentials() {
		t.Error("Expected B2 client to have credentials")
	}
}

// TestNewEnrichmentService_NoStorage tests initialization with no storage configured
func TestNewEnrichmentService_NoStorage(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		PostgresHost:          "localhost",
		PostgresPort:          5432,
		PostgresUser:          "test",
		PostgresPassword:      "test",
		PostgresDatabase:      "test",
		DatabaseName:          "test",
		LLMBaseURL:            "https://api.test.com",
		LLMAPIKey:             "test-key",
		LLMModel:              "test-model",
		// Neither R2 nor B2 configured
		R2AccessKeyID:     "",
		R2SecretAccessKey: "",
		R2Endpoint:        "",
		R2BucketName:      "",
		B2AccessKeyID:     "",
		B2SecretAccessKey: "",
		B2Endpoint:        "",
		B2BucketName:      "",
		MinTurnCount:          100,
		MinWinProbCrossings:   3,
		UpsetThreshold:        150.0,
		MaxEnrichmentsPerHour: 20,
		MaxConcurrentRequests: 3,
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.store == nil {
		t.Error("Expected store to be initialized")
	}
	if svc.selector == nil {
		t.Error("Expected selector to be initialized")
	}
	if svc.generator == nil {
		t.Error("Expected generator to be initialized")
	}
	if svc.r2Client == nil {
		t.Error("Expected r2Client to be initialized (even without credentials)")
	}
	if svc.b2Client == nil {
		t.Error("Expected b2Client to be initialized (even without credentials)")
	}
	if svc.llmClient == nil {
		t.Error("Expected llmClient to be initialized")
	}

	// Verify that neither client has credentials
	if svc.r2Client.HasCredentials() {
		t.Error("Expected R2 client to have no credentials")
	}
	if svc.b2Client.HasCredentials() {
		t.Error("Expected B2 client to have no credentials")
	}
}

// TestNewEnrichmentService_MinimalConfig tests with minimal configuration
func TestNewEnrichmentService_MinimalConfig(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		// Only required fields - use defaults for everything else
		PostgresHost:     "localhost",
		PostgresDatabase: "test",
		LLMBaseURL:       "https://api.test.com",
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.db == nil {
		t.Error("Expected db to be set")
	}
	if svc.store == nil {
		t.Error("Expected store to be initialized")
	}
	if svc.selector == nil {
		t.Error("Expected selector to be initialized")
	}
	if svc.generator == nil {
		t.Error("Expected generator to be initialized")
	}
	if svc.r2Client == nil {
		t.Error("Expected r2Client to be initialized")
	}
	if svc.b2Client == nil {
		t.Error("Expected b2Client to be initialized")
	}
	if svc.llmClient == nil {
		t.Error("Expected llmClient to be initialized")
	}
}

// TestCycleResults verifies the CycleResults struct can be properly used
func TestCycleResults(t *testing.T) {
	results := CycleResults{
		Processed: 10,
		Enriched:  8,
		Skipped:   5,
		Failed:    2,
	}

	if results.Processed != 10 {
		t.Errorf("Expected Processed=10, got %d", results.Processed)
	}
	if results.Enriched != 8 {
		t.Errorf("Expected Enriched=8, got %d", results.Enriched)
	}
	if results.Skipped != 5 {
		t.Errorf("Expected Skipped=5, got %d", results.Skipped)
	}
	if results.Failed != 2 {
		t.Errorf("Expected Failed=2, got %d", results.Failed)
	}

	// Verify the relationship: Processed >= Enriched + Failed
	if results.Processed < (results.Enriched + results.Failed) {
		t.Error("Processed should be >= Enriched + Failed")
	}
}

// TestNewEnrichmentService_SelectorConfig tests that selector config is properly passed
func TestNewEnrichmentService_SelectorConfig(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		PostgresHost:     "localhost",
		PostgresPort:     5432,
		PostgresUser:     "test",
		PostgresPassword: "test",
		PostgresDatabase: "test",
		DatabaseName:     "test",
		LLMBaseURL:       "https://api.test.com",
		LLMAPIKey:        "test-key",
		LLMModel:         "test-model",
		R2AccessKeyID:    "r2-key",
		R2SecretAccessKey: "r2-secret",
		R2Endpoint:       "https://r2.test.com",
		R2BucketName:     "test-bucket",
		B2AccessKeyID:    "b2-key",
		B2SecretAccessKey: "b2-secret",
		B2Endpoint:       "https://b2.test.com",
		B2BucketName:     "test-bucket",
		MinTurnCount:      150,
		MinWinProbCrossings: 5,
		UpsetThreshold:    200.0,
		MaxEnrichmentsPerHour: 25,
		MaxConcurrentRequests: 4,
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.selector == nil {
		t.Fatal("Expected selector to be initialized")
	}
	// The selector should be configured with the config values
	// We can't directly inspect the selector's config, but we verify it's not nil
}

// TestNewEnrichmentService_GeneratorConfig tests that generator config is properly passed
func TestNewEnrichmentService_GeneratorConfig(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		PostgresHost:          "localhost",
		PostgresPort:          5432,
		PostgresUser:          "test",
		PostgresPassword:      "test",
		PostgresDatabase:      "test",
		DatabaseName:          "test",
		LLMBaseURL:            "https://api.test.com",
		LLMAPIKey:             "test-key",
		LLMModel:              "test-model",
		R2AccessKeyID:         "r2-key",
		R2SecretAccessKey:     "r2-secret",
		R2Endpoint:            "https://r2.test.com",
		R2BucketName:          "test-bucket",
		B2AccessKeyID:         "b2-key",
		B2SecretAccessKey:     "b2-secret",
		B2Endpoint:            "https://b2.test.com",
		B2BucketName:          "test-bucket",
		MinTurnCount:          100,
		MinWinProbCrossings:   3,
		UpsetThreshold:        150.0,
		MaxEnrichmentsPerHour: 20,
		MaxConcurrentRequests: 5,
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.generator == nil {
		t.Fatal("Expected generator to be initialized")
	}
	// The generator should be configured with the config values
	// We can't directly inspect the generator's config, but we verify it's not nil
}
