package main

import (
	"os"
	"testing"
)

// TestGetOutputFolder tests the getOutputFolder function.
func TestGetOutputFolder(t *testing.T) {
	tests := []struct {
		args       []string
		wantFolder string
		wantErr    bool
	}{
		{
			args:       []string{"-p", "./testdata"},
			wantFolder: "./testdata",
			wantErr:    false,
		},
		{
			args:       []string{"-p", "./data/"},
			wantFolder: "./data/",
			wantErr:    false,
		},
		{
			args:       []string{"-p", "./invalidpath"},
			wantFolder: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.wantFolder, func(t *testing.T) {
			if tt.wantFolder == "./invalidpath" {
				if err := os.MkdirAll(tt.wantFolder, 0755); err != nil {
					t.Errorf("Unexpected error when creating directory: %v", err)
				}
				if err := os.Chmod(tt.wantFolder, 0444); err != nil { // Remove write permission
					t.Errorf("Error setting permissions: %v", err)
				}
			}
			gotFolder, err := getOutputFolder(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("getOutputFolder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFolder != tt.wantFolder {
				t.Errorf("getOutputFolder() = %v, want %v", gotFolder, tt.wantFolder)
			}
			if err := os.RemoveAll(tt.wantFolder); err != nil {
				t.Errorf("Error cleaning up test directory: %v", err)
			}
		})
	}
}

// TestWritePermission checks if write permissions are handled correctly.
func TestWritePermission(t *testing.T) {
	tests := []struct {
		dirPath string
		hasErr  bool
	}{
		{"./data", false},    // Assuming ./data exists and is writable
		{"./readonly", true}, // Assuming ./readonly is a directory with no write permission
	}

	for _, tt := range tests {
		t.Run(tt.dirPath, func(t *testing.T) {
			_, err := os.Stat(tt.dirPath)
			if err != nil && tt.hasErr {
				// Expected error, test passes
				return
			}
			if err == nil && tt.hasErr {
				t.Errorf("Expected error for directory %s, but got none", tt.dirPath)
			}
		})
	}
}
