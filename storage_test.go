package farp

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"testing"
)

// Mock storage backend for testing
type mockStorageBackend struct {
	data    map[string][]byte
	watches map[string][]chan StorageEvent
}

func newMockStorageBackend() *mockStorageBackend {
	return &mockStorageBackend{
		data:    make(map[string][]byte),
		watches: make(map[string][]chan StorageEvent),
	}
}

func (m *mockStorageBackend) Put(ctx context.Context, key string, value []byte) error {
	m.data[key] = value
	// Notify watchers
	for prefix, channels := range m.watches {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			for _, ch := range channels {
				select {
				case ch <- StorageEvent{Type: EventTypeAdded, Key: key, Value: value}:
				default:
				}
			}
		}
	}
	return nil
}

func (m *mockStorageBackend) Get(ctx context.Context, key string) ([]byte, error) {
	value, ok := m.data[key]
	if !ok {
		return nil, ErrSchemaNotFound
	}
	return value, nil
}

func (m *mockStorageBackend) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	// Notify watchers
	for prefix, channels := range m.watches {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			for _, ch := range channels {
				select {
				case ch <- StorageEvent{Type: EventTypeRemoved, Key: key}:
				default:
				}
			}
		}
	}
	return nil
}

func (m *mockStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	keys := []string{}
	for key := range m.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (m *mockStorageBackend) Watch(ctx context.Context, prefix string) (<-chan StorageEvent, error) {
	ch := make(chan StorageEvent, 10)
	m.watches[prefix] = append(m.watches[prefix], ch)
	
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	
	return ch, nil
}

func (m *mockStorageBackend) Close() error {
	return nil
}

func TestNewStorageHelper(t *testing.T) {
	backend := newMockStorageBackend()
	helper := NewStorageHelper(backend, 1024, 10240)

	if helper.backend != backend {
		t.Error("StorageHelper has wrong backend")
	}

	if helper.compressionThreshold != 1024 {
		t.Errorf("compressionThreshold = %d, want 1024", helper.compressionThreshold)
	}

	if helper.maxSize != 10240 {
		t.Errorf("maxSize = %d, want 10240", helper.maxSize)
	}
}

func TestStorageHelper_PutJSON_GetJSON(t *testing.T) {
	backend := newMockStorageBackend()
	helper := NewStorageHelper(backend, 1024, 10240)

	ctx := context.Background()
	key := "test-key"
	value := map[string]interface{}{
		"name": "test",
		"age":  30,
	}

	// Put JSON
	err := helper.PutJSON(ctx, key, value)
	if err != nil {
		t.Fatalf("PutJSON() error = %v", err)
	}

	// Get JSON
	var result map[string]interface{}
	err = helper.GetJSON(ctx, key, &result)
	if err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("result[name] = %v, want test", result["name"])
	}

	if result["age"].(float64) != 30 {
		t.Errorf("result[age] = %v, want 30", result["age"])
	}
}

func TestStorageHelper_PutJSON_MaxSize(t *testing.T) {
	backend := newMockStorageBackend()
	helper := NewStorageHelper(backend, 1024, 100) // Max 100 bytes

	ctx := context.Background()
	key := "test-key"
	
	// Create a large value that exceeds max size
	largeValue := map[string]interface{}{
		"data": string(make([]byte, 200)),
	}

	err := helper.PutJSON(ctx, key, largeValue)
	if err == nil {
		t.Error("PutJSON() should fail for data exceeding max size")
	}

	if !errors.Is(err, ErrSchemaToLarge) {
		t.Errorf("expected ErrSchemaToLarge, got %v", err)
	}
}

func TestStorageHelper_PutJSON_Compression(t *testing.T) {
	backend := newMockStorageBackend()
	helper := NewStorageHelper(backend, 50, 10240) // Compress data > 50 bytes

	ctx := context.Background()
	key := "test-key"
	
	// Create a value that triggers compression
	value := map[string]interface{}{
		"data": "this is a long string that should trigger compression",
	}

	err := helper.PutJSON(ctx, key, value)
	if err != nil {
		t.Fatalf("PutJSON() error = %v", err)
	}

	// Verify compressed version exists
	compressedKey := key + ".gz"
	compressedData, err := backend.Get(ctx, compressedKey)
	if err != nil {
		t.Error("Expected compressed data to be stored")
	}

	// Verify it's actually gzip compressed
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		t.Errorf("Data should be gzip compressed: %v", err)
	}
	reader.Close()

	// Verify we can retrieve it
	var result map[string]interface{}
	err = helper.GetJSON(ctx, key, &result)
	if err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}

	if result["data"] != "this is a long string that should trigger compression" {
		t.Error("Retrieved data doesn't match original")
	}
}

func TestStorageHelper_GetJSON_NotFound(t *testing.T) {
	backend := newMockStorageBackend()
	helper := NewStorageHelper(backend, 1024, 10240)

	ctx := context.Background()
	var result map[string]interface{}

	err := helper.GetJSON(ctx, "nonexistent", &result)
	if err == nil {
		t.Error("GetJSON() should return error for nonexistent key")
	}

	if !errors.Is(err, ErrSchemaNotFound) {
		t.Errorf("expected ErrSchemaNotFound, got %v", err)
	}
}

func TestCompressDecompressData(t *testing.T) {
	original := []byte("this is test data that we want to compress and decompress")

	// Compress
	compressed, err := compressData(original)
	if err != nil {
		t.Fatalf("compressData() error = %v", err)
	}

	if len(compressed) == 0 {
		t.Error("compressData() returned empty data")
	}

	// Verify it's smaller (for reasonable data size)
	// Note: Very small data might not compress well
	if len(compressed) > len(original)*2 {
		t.Error("compressed data should not be much larger than original")
	}

	// Decompress
	decompressed, err := decompressData(compressed)
	if err != nil {
		t.Fatalf("decompressData() error = %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Error("decompressed data doesn't match original")
	}
}

func TestNewManifestStorage(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "test-namespace", 1024, 10240)

	if storage.namespace != "test-namespace" {
		t.Errorf("namespace = %v, want test-namespace", storage.namespace)
	}

	if storage.helper == nil {
		t.Error("helper should be initialized")
	}
}

func TestManifestStorage_PutGet(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"

	// Put manifest
	err := storage.Put(ctx, manifest)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Get manifest
	retrieved, err := storage.Get(ctx, "test-service", "instance-123")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.ServiceName != manifest.ServiceName {
		t.Errorf("retrieved service name = %v, want %v", retrieved.ServiceName, manifest.ServiceName)
	}

	if retrieved.InstanceID != manifest.InstanceID {
		t.Errorf("retrieved instance ID = %v, want %v", retrieved.InstanceID, manifest.InstanceID)
	}
}

func TestManifestStorage_GetNotFound(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()
	_, err := storage.Get(ctx, "nonexistent", "instance-123")
	if err == nil {
		t.Error("Get() should return error for nonexistent manifest")
	}

	if !errors.Is(err, ErrManifestNotFound) {
		t.Errorf("expected ErrManifestNotFound, got %v", err)
	}
}

func TestManifestStorage_Delete(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"

	// Put manifest
	err := storage.Put(ctx, manifest)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Delete manifest
	err = storage.Delete(ctx, "test-service", "instance-123")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's deleted
	_, err = storage.Get(ctx, "test-service", "instance-123")
	if !errors.Is(err, ErrManifestNotFound) {
		t.Error("Manifest should be deleted")
	}
}

func TestManifestStorage_List(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()

	// Add multiple manifests
	manifest1 := NewManifest("test-service", "v1.0.0", "instance-1")
	manifest1.Endpoints.Health = "/health"
	manifest2 := NewManifest("test-service", "v1.0.0", "instance-2")
	manifest2.Endpoints.Health = "/health"

	storage.Put(ctx, manifest1)
	storage.Put(ctx, manifest2)

	// List manifests
	manifests, err := storage.List(ctx, "test-service")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(manifests) != 2 {
		t.Errorf("List() returned %d manifests, want 2", len(manifests))
	}
}

func TestManifestStorage_PutGetSchema(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()
	path := "/schemas/test/v1/openapi"
	schema := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	// Put schema
	err := storage.PutSchema(ctx, path, schema)
	if err != nil {
		t.Fatalf("PutSchema() error = %v", err)
	}

	// Get schema
	retrieved, err := storage.GetSchema(ctx, path)
	if err != nil {
		t.Fatalf("GetSchema() error = %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("retrieved schema is not map[string]interface{}")
	}

	if retrievedMap["openapi"] != "3.1.0" {
		t.Errorf("retrieved openapi version = %v, want 3.1.0", retrievedMap["openapi"])
	}
}

func TestManifestStorage_DeleteSchema(t *testing.T) {
	backend := newMockStorageBackend()
	storage := NewManifestStorage(backend, "farp", 1024, 10240)

	ctx := context.Background()
	path := "/schemas/test/v1/openapi"
	schema := map[string]interface{}{"test": "data"}

	// Put schema
	storage.PutSchema(ctx, path, schema)

	// Delete schema
	err := storage.DeleteSchema(ctx, path)
	if err != nil {
		t.Fatalf("DeleteSchema() error = %v", err)
	}

	// Verify it's deleted
	_, err = storage.GetSchema(ctx, path)
	if !errors.Is(err, ErrSchemaNotFound) {
		t.Error("Schema should be deleted")
	}
}

