package formats

import (
	"testing"
	"time"
)

func TestCiscoUmbrellaFormat_Name(t *testing.T) {
	format := NewCiscoUmbrellaFormat()
	if format.Name() != "cisco_umbrella" {
		t.Errorf("Expected name 'cisco_umbrella', got '%s'", format.Name())
	}
}

func TestCiscoUmbrellaFormat_ParseTimestamp(t *testing.T) {
	format := NewCiscoUmbrellaFormat()

	tests := []struct {
		filename  string
		wantUnix  int64
		expectErr bool
	}{
		{
			filename:  "2024-01-15-10-30-0001.csv.gz",
			wantUnix:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).Unix(),
			expectErr: false,
		},
		{
			filename:  "2024-12-31-23-59-9999.csv.gz",
			wantUnix:  time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC).Unix(),
			expectErr: false,
		},
		{
			filename:  "2024-01-15-10-30-0001.csv", // No .gz extension
			wantUnix:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).Unix(),
			expectErr: false,
		},
		{
			filename:  "invalid-filename.csv.gz",
			wantUnix:  0,
			expectErr: true,
		},
		{
			filename:  "2024-13-15-10-30-0001.csv.gz", // Invalid month, but we don't validate ranges
			wantUnix:  time.Date(2024, 13, 15, 10, 30, 0, 0, time.UTC).Unix(),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := format.ParseTimestamp(tt.filename)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for filename %s, got none", tt.filename)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error for filename %s: %v", tt.filename, err)
				return
			}
			if got != tt.wantUnix {
				t.Errorf("ParseTimestamp(%s) = %d, want %d", tt.filename, got, tt.wantUnix)
			}
		})
	}
}

func TestCiscoUmbrellaFormat_ProcessContent(t *testing.T) {
	format := NewCiscoUmbrellaFormat()

	tests := []struct {
		name        string
		line        string
		isFirstLine bool
		want        string
		expectNil   bool
	}{
		{
			name:        "header line skipped",
			line:        "timestamp,domain,action,identity,categories",
			isFirstLine: true,
			want:        "",
			expectNil:   true,
		},
		{
			name:        "data line passed through",
			line:        "2024-01-15 10:00:00,example.com,allow,user1,Business",
			isFirstLine: false,
			want:        "2024-01-15 10:00:00,example.com,allow,user1,Business",
			expectNil:   false,
		},
		{
			name:        "empty line skipped",
			line:        "",
			isFirstLine: false,
			want:        "",
			expectNil:   true,
		},
		{
			name:        "whitespace only line skipped",
			line:        "   ",
			isFirstLine: false,
			want:        "",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := format.ProcessContent([]byte(tt.line), tt.isFirstLine)
			if err != nil {
				t.Errorf("ProcessContent() error = %v", err)
				return
			}
			if tt.expectNil {
				if got != nil {
					t.Errorf("ProcessContent() = %q, want nil", string(got))
				}
				return
			}
			if got == nil {
				t.Errorf("ProcessContent() = nil, want %q", tt.want)
				return
			}
			if string(got) != tt.want {
				t.Errorf("ProcessContent() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestCiscoUmbrellaFormat_GetContentType(t *testing.T) {
	format := NewCiscoUmbrellaFormat()
	if format.GetContentType() != "text/plain" {
		t.Errorf("GetContentType() = %q, want 'text/plain'", format.GetContentType())
	}
}

func TestCiscoUmbrellaFormat_DetectFromFilename(t *testing.T) {
	format := NewCiscoUmbrellaFormat()

	tests := []struct {
		filename string
		want     bool
	}{
		{"2024-01-15-10-30-0001.csv.gz", true},
		{"2024-12-31-23-59-9999.csv.gz", true},
		{"2024-01-15-10-30-0001.csv", true},           // No .gz
		{"1705315200_12345_67890_001.json.gz", false}, // Zscaler format
		{"random_file.txt", false},
		{"2024-13-15-10-30-0001.csv.gz", true}, // Still matches pattern even if invalid date
		{"2024-01-15.csv.gz", false},           // Too few parts
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := format.DetectFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("DetectFromFilename(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestCiscoUmbrellaFormat_DetectFromContent(t *testing.T) {
	format := NewCiscoUmbrellaFormat()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "valid cisco umbrella csv",
			content: `timestamp,domain,action,identity,categories
2024-01-15 10:00:00,example.com,allow,user1,Business
2024-01-15 10:00:01,test.com,block,user2,Malware`,
			want: true,
		},
		{
			name: "missing header keywords",
			content: `date,site,status,user,tags
2024-01-15 10:00:00,example.com,allow,user1,Business`,
			want: false,
		},
		{
			name:    "empty content",
			content: "",
			want:    false,
		},
		{
			name:    "single line",
			content: "timestamp,domain,action",
			want:    false,
		},
		{
			name: "json content",
			content: `{"timestamp": "2024-01-15", "domain": "example.com"}
{"timestamp": "2024-01-15", "domain": "test.com"}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.DetectFromContent([]byte(tt.content))
			if got != tt.want {
				t.Errorf("DetectFromContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_DetectFormat(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name     string
		filename string
		content  string
		want     string
	}{
		{
			name:     "cisco umbrella filename",
			filename: "2024-01-15-10-30-0001.csv.gz",
			content:  "",
			want:     "cisco_umbrella",
		},
		{
			name:     "zscaler filename",
			filename: "1705315200_12345_67890_001.json.gz",
			content:  "",
			want:     "zscaler",
		},
		{
			name:     "unknown filename falls back to zscaler",
			filename: "unknown.txt",
			content:  "",
			want:     "zscaler",
		},
		{
			name:     "cisco umbrella content detection",
			filename: "unknown.csv.gz",
			content: `timestamp,domain,action,identity,categories
2024-01-15 10:00:00,example.com,allow,user1,Business`,
			want: "cisco_umbrella",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.DetectFormat(tt.filename, []byte(tt.content))
			if got == nil {
				t.Errorf("DetectFormat() returned nil")
				return
			}
			if got.Name() != tt.want {
				t.Errorf("DetectFormat() = %s, want %s", got.Name(), tt.want)
			}
		})
	}
}
