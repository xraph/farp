package farp

import (
	"errors"
	"testing"
)

func TestManifestError_Error(t *testing.T) {
	baseErr := errors.New("test error")
	manifestErr := &ManifestError{
		ServiceName: "test-service",
		InstanceID:  "instance-123",
		Err:         baseErr,
	}

	expected := "manifest error for service=test-service instance=instance-123: test error"
	if got := manifestErr.Error(); got != expected {
		t.Errorf("ManifestError.Error() = %v, want %v", got, expected)
	}
}

func TestManifestError_Unwrap(t *testing.T) {
	baseErr := errors.New("test error")
	manifestErr := &ManifestError{
		ServiceName: "test-service",
		InstanceID:  "instance-123",
		Err:         baseErr,
	}

	unwrapped := manifestErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("ManifestError.Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	if !errors.Is(manifestErr, baseErr) {
		t.Error("ManifestError should wrap baseErr")
	}
}

func TestSchemaError_Error(t *testing.T) {
	baseErr := errors.New("test error")
	schemaErr := &SchemaError{
		Type: SchemaTypeOpenAPI,
		Path: "/schemas/test/v1",
		Err:  baseErr,
	}

	expected := "schema error type=openapi path=/schemas/test/v1: test error"
	if got := schemaErr.Error(); got != expected {
		t.Errorf("SchemaError.Error() = %v, want %v", got, expected)
	}
}

func TestSchemaError_Unwrap(t *testing.T) {
	baseErr := errors.New("test error")
	schemaErr := &SchemaError{
		Type: SchemaTypeOpenAPI,
		Path: "/schemas/test/v1",
		Err:  baseErr,
	}

	unwrapped := schemaErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("SchemaError.Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	if !errors.Is(schemaErr, baseErr) {
		t.Error("SchemaError should wrap baseErr")
	}
}

func TestValidationError_Error(t *testing.T) {
	validationErr := &ValidationError{
		Field:   "service_name",
		Message: "service name is required",
	}

	expected := "validation error: field=service_name message=service name is required"
	if got := validationErr.Error(); got != expected {
		t.Errorf("ValidationError.Error() = %v, want %v", got, expected)
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that all error constants are defined and not nil
	errorConstants := []error{
		ErrManifestNotFound,
		ErrSchemaNotFound,
		ErrInvalidManifest,
		ErrInvalidSchema,
		ErrSchemaToLarge,
		ErrChecksumMismatch,
		ErrUnsupportedType,
		ErrBackendUnavailable,
		ErrIncompatibleVersion,
		ErrInvalidLocation,
		ErrProviderNotFound,
		ErrRegistryNotConfigured,
		ErrSchemaFetchFailed,
		ErrValidationFailed,
	}

	for i, err := range errorConstants {
		if err == nil {
			t.Errorf("error constant at index %d is nil", i)
		}
		if err.Error() == "" {
			t.Errorf("error constant at index %d has empty message", i)
		}
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test that errors.Is works with custom error types
	baseErr := errors.New("base error")

	manifestErr := &ManifestError{
		ServiceName: "test",
		InstanceID:  "123",
		Err:         baseErr,
	}

	if !errors.Is(manifestErr, baseErr) {
		t.Error("ManifestError should be identifiable with errors.Is")
	}

	schemaErr := &SchemaError{
		Type: SchemaTypeOpenAPI,
		Path: "/test",
		Err:  baseErr,
	}

	if !errors.Is(schemaErr, baseErr) {
		t.Error("SchemaError should be identifiable with errors.Is")
	}
}
