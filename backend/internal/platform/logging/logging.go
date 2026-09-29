// Package logging builds the structured logger used by every binary.
//
// Logs are JSON lines on stdout. Cloud Run forwards stdout to Cloud Logging,
// which parses each JSON line into a structured entry, so fields such as
// "status" or "duration" become searchable without any extra agent.
package logging

import (
	"io"
	"log/slog"
)

// New returns a JSON logger that writes to w and drops records below level.
//
// Two keys are renamed to the names Cloud Logging understands, so the log
// viewer shows the right severity and message text:
//
//	level -> severity (and WARN -> WARNING)
//	msg   -> message
func New(w io.Writer, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: cloudLoggingKeys,
	})
	return slog.New(handler)
}

// cloudLoggingKeys renames slog's built-in keys to Cloud Logging's.
// See https://cloud.google.com/logging/docs/structured-logging.
func cloudLoggingKeys(groups []string, a slog.Attr) slog.Attr {
	// Only top-level attributes can be the built-in ones.
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
		if level, ok := a.Value.Any().(slog.Level); ok && level == slog.LevelWarn {
			a.Value = slog.StringValue("WARNING")
		}
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}
