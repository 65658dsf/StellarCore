package log

import (
	"strings"
	"testing"
	"time"

	goliblog "github.com/fatedier/golib/log"
)

func TestLogBufferQueryFiltersAndCursor(t *testing.T) {
	ResetBufferForTesting()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	bufferLog.add([]byte("first\n"), goliblog.InfoLevel, now)
	bufferLog.add([]byte("second\n"), goliblog.WarnLevel, now)
	bufferLog.add([]byte("third\n"), goliblog.ErrorLevel, now)

	result := QueryEntries(1, 10, "warn")
	if len(result.Entries) != 1 {
		t.Fatalf("expected one warn entry, got %d", len(result.Entries))
	}
	if result.Entries[0].ID != 2 || result.Entries[0].Level != "warn" || !strings.Contains(result.Entries[0].Message, "second") {
		t.Fatalf("unexpected entry %#v", result.Entries[0])
	}
	if result.NextCursor != 2 {
		t.Fatalf("next cursor = %d, want 2", result.NextCursor)
	}
}

func TestLogBufferQueryTruncatesToLimit(t *testing.T) {
	ResetBufferForTesting()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		bufferLog.add([]byte("line\n"), goliblog.InfoLevel, now)
	}

	result := QueryEntries(0, 2, "")
	if !result.Truncated {
		t.Fatalf("expected truncated result")
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].ID != 4 || result.Entries[1].ID != 5 {
		t.Fatalf("expected latest two entries, got %#v", result.Entries)
	}
	if result.NextCursor != 5 {
		t.Fatalf("next cursor = %d, want 5", result.NextCursor)
	}
}
