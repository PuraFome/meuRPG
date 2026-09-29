package config

import (
	"log/slog"
	"strings"
	"testing"
)

// env turns a map into a getenv function, so each test case declares its
// environment in one place.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string // substrings expected in the error; nil means success
	}{
		{
			name: "defaults when nothing is set",
			env:  map[string]string{},
			want: Config{Port: 8080, LogLevel: slog.LevelInfo},
		},
		{
			name: "all variables set",
			env: map[string]string{
				"PORT":         "9090",
				"DATABASE_URL": "postgresql://root@localhost:26257/meurpg?sslmode=disable",
				"LOG_LEVEL":    "DEBUG",
			},
			want: Config{
				Port:        9090,
				DatabaseURL: "postgresql://root@localhost:26257/meurpg?sslmode=disable",
				LogLevel:    slog.LevelDebug,
			},
		},
		{
			name: "surrounding whitespace is ignored",
			env:  map[string]string{"PORT": " 3000 ", "LOG_LEVEL": " warn ", "DATABASE_URL": "  "},
			want: Config{Port: 3000, LogLevel: slog.LevelWarn},
		},
		{
			name:    "port is not a number",
			env:     map[string]string{"PORT": "http"},
			wantErr: []string{"PORT must be a number"},
		},
		{
			name:    "port out of range",
			env:     map[string]string{"PORT": "70000"},
			wantErr: []string{"PORT must be between 1 and 65535"},
		},
		{
			name:    "unknown log level",
			env:     map[string]string{"LOG_LEVEL": "verbose"},
			wantErr: []string{"LOG_LEVEL must be one of"},
		},
		{
			name:    "every problem is reported at once",
			env:     map[string]string{"PORT": "0", "LOG_LEVEL": "loud"},
			wantErr: []string{"PORT", "LOG_LEVEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Load(env(tt.env))

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Load() error = nil, want an error containing %q", tt.wantErr)
				}
				for _, sub := range tt.wantErr {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("Load() error = %q, want it to contain %q", err, sub)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
