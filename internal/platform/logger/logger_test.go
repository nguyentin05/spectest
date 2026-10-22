package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestNew_LevelParsing(t *testing.T) {
	tests := []struct {
		input       string
		wantEnabled slog.Level
		wantHidden  slog.Level
		hasHidden   bool
	}{
		{input: "debug", wantEnabled: slog.LevelDebug},
		{input: "INFO", wantEnabled: slog.LevelInfo, wantHidden: slog.LevelDebug, hasHidden: true},
		{input: " warn ", wantEnabled: slog.LevelWarn, wantHidden: slog.LevelInfo, hasHidden: true},
		{input: "error", wantEnabled: slog.LevelError, wantHidden: slog.LevelWarn, hasHidden: true},
		{input: "unknown", wantEnabled: slog.LevelInfo, wantHidden: slog.LevelDebug, hasHidden: true},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			if !l.Enabled(ctx, tt.wantEnabled) {
				t.Errorf("level %v should be enabled for input %q", tt.wantEnabled, tt.input)
			}
			if tt.hasHidden && l.Enabled(ctx, tt.wantHidden) {
				t.Errorf("level %v should be disabled for input %q", tt.wantHidden, tt.input)
			}
		})
	}
}
