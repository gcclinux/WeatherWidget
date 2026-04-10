package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 3: Corrupt configuration fallback**
// **Validates: Requirements 8.3**

func TestProperty3_CorruptConfigFallback(t *testing.T) {
	expected := DefaultConfig()
	iteration := 0
	baseDir := t.TempDir()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random byte sequences
		corruptData := rapid.SliceOf(rapid.Byte()).Draw(rt, "corruptBytes")

		tmpDir := filepath.Join(baseDir, fmt.Sprintf("iter_%d", iteration))
		iteration++

		svc := NewConfigService(tmpDir)

		// Create the config directory structure
		configDir := filepath.Dir(svc.ConfigPath())
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			rt.Fatalf("MkdirAll error: %v", err)
		}

		// Write corrupt data to the config file
		if err := os.WriteFile(svc.ConfigPath(), corruptData, 0o644); err != nil {
			rt.Fatalf("WriteFile error: %v", err)
		}

		// Load should return DefaultConfig without panic
		loaded, err := svc.Load()
		if err != nil {
			rt.Fatalf("Load() returned unexpected error: %v", err)
		}

		if !reflect.DeepEqual(expected, loaded) {
			rt.Fatalf("Expected DefaultConfig but got: %+v", loaded)
		}
	})
}

func TestProperty3_CorruptConfigFallback_EmptyBytes(t *testing.T) {
	expected := DefaultConfig()
	tmpDir := t.TempDir()
	svc := NewConfigService(tmpDir)

	configDir := filepath.Dir(svc.ConfigPath())
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	// Write empty bytes
	if err := os.WriteFile(svc.ConfigPath(), []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	loaded, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if !reflect.DeepEqual(expected, loaded) {
		t.Fatalf("Expected DefaultConfig but got: %+v", loaded)
	}
}

func TestProperty3_CorruptConfigFallback_PartialJSON(t *testing.T) {
	expected := DefaultConfig()
	partials := []string{
		`{"dataSource":`,
		`{"cities":[{"name":"Test"`,
		`{"refreshInterval": 10,`,
		`{`,
		`[`,
		`{"dataSource": "remote_api", "cities": [`,
	}

	for i, partial := range partials {
		t.Run(fmt.Sprintf("partial_%d", i), func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewConfigService(tmpDir)

			configDir := filepath.Dir(svc.ConfigPath())
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatalf("MkdirAll error: %v", err)
			}

			if err := os.WriteFile(svc.ConfigPath(), []byte(partial), 0o644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}

			loaded, err := svc.Load()
			if err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}

			if !reflect.DeepEqual(expected, loaded) {
				t.Fatalf("Expected DefaultConfig for partial JSON %q but got: %+v", partial, loaded)
			}
		})
	}
}

func TestProperty3_CorruptConfigFallback_WrongSchema(t *testing.T) {
	expected := DefaultConfig()
	wrongSchemas := []string{
		`{"foo": "bar"}`,
		`{"name": 123, "items": [1,2,3]}`,
		`"just a string"`,
		`42`,
		`true`,
		`null`,
		`[]`,
	}

	for i, schema := range wrongSchemas {
		t.Run(fmt.Sprintf("wrong_schema_%d", i), func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewConfigService(tmpDir)

			configDir := filepath.Dir(svc.ConfigPath())
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatalf("MkdirAll error: %v", err)
			}

			if err := os.WriteFile(svc.ConfigPath(), []byte(schema), 0o644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}

			loaded, err := svc.Load()
			if err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}

			if !reflect.DeepEqual(expected, loaded) {
				t.Fatalf("Expected DefaultConfig for wrong schema %q but got: %+v", schema, loaded)
			}
		})
	}
}

func TestProperty3_CorruptConfigFallback_BinaryData(t *testing.T) {
	expected := DefaultConfig()
	binaryInputs := [][]byte{
		{0x00, 0xFF, 0xFE, 0xFD, 0x80, 0x7F},
		{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG header
		{0x25, 0x50, 0x44, 0x46},                         // PDF header
	}

	for i, bin := range binaryInputs {
		t.Run(fmt.Sprintf("binary_%d", i), func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewConfigService(tmpDir)

			configDir := filepath.Dir(svc.ConfigPath())
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatalf("MkdirAll error: %v", err)
			}

			if err := os.WriteFile(svc.ConfigPath(), bin, 0o644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}

			loaded, err := svc.Load()
			if err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}

			if !reflect.DeepEqual(expected, loaded) {
				t.Fatalf("Expected DefaultConfig for binary data but got: %+v", loaded)
			}
		})
	}
}
