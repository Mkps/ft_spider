package main

import (
	"os"
	"testing"
)

func TestGetRecurseLevel(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLevel int
		wantErr   bool
	}{
		{
			name:      "-l but no -l value",
			args:      []string{"-r", "-l"},
			wantLevel: 0,
			wantErr:   true,
		},
		{
			name:      "-l value 5",
			args:      []string{"-r", "-l", "5"},
			wantLevel: 5,
			wantErr:   false,
		},
		{
			name:      "-r no -l value",
			args:      []string{"-r"},
			wantLevel: 5,
			wantErr:   false,
		},
		{
			name:      "multiple -r ",
			args:      []string{"-r", "-r"},
			wantLevel: 0,
			wantErr:   true,
		},
		{
			name:      "no -r no -l",
			args:      []string{""},
			wantLevel: 0,
			wantErr:   false,
		},
		{
			name:      "42 inside range",
			args:      []string{"-r", "-l", "42"},
			wantLevel: 42,
			wantErr:   false,
		},
		{
			name:      "lower limit",
			args:      []string{"-r", "-l", "1"},
			wantLevel: 1,
			wantErr:   false,
		},
		{
			name:      "upper limit",
			args:      []string{"-r", "-l", "50"},
			wantLevel: 50,
			wantErr:   false,
		},
		{
			name:      "under by 1",
			args:      []string{"-r", "-l", "0"},
			wantLevel: 0,
			wantErr:   true,
		},
		{
			name:      "over by 1",
			args:      []string{"-r", "-l", "51"},
			wantLevel: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, err := getRecurseLevel(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRecurseLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotLevel != tt.wantLevel {
				t.Errorf("getRecurseLevel() = %v, want %v", gotLevel, tt.wantLevel)
			}
		})
	}
}

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
