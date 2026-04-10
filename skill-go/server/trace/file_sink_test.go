package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTestEvent() FlowEvent {
	return FlowEvent{
		FlowID:    1,
		Timestamp: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		Span:      "spell",
		Event:     "prepare",
		SpellID:   1001,
		SpellName: "Fireball",
		Fields:    map[string]interface{}{"target": "Warrior"},
	}
}

func TestFileSink_WritesJSONLines(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	fs.Write(makeTestEvent())
	fs.Close()

	// Read file and verify
	files, _ := filepath.Glob(filepath.Join(dir, "trace-*.log"))
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(files))
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var parsed fileEventJSON
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed.Span != "spell" || parsed.Event != "prepare" {
		t.Errorf("unexpected span.event: %s.%s", parsed.Span, parsed.Event)
	}
	if parsed.SpellName != "Fireball" {
		t.Errorf("expected spellName=Fireball, got %s", parsed.SpellName)
	}
}

func TestFileSink_DailyRotation(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	// Write an event to today's file
	fs.Write(makeTestEvent())

	// Simulate date change by directly manipulating state
	oldFile := fs.file
	fs.file = nil // prevent close from operating on old file

	// openFile with a different date name
	err = fs.openFile("2025-01-01")
	if err != nil {
		t.Fatalf("openFile: %v", err)
	}
	oldFile.Close()

	fs.Write(makeTestEvent())
	fs.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "trace-*.log"))
	if len(files) != 2 {
		t.Fatalf("expected 2 log files after rotation, got %d: %v", len(files), files)
	}
}

func TestFileSink_NonBlockingWrite(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	// Fill the channel (capacity 4096) — should not block
	droppedBefore := fs.Dropped()
	for i := 0; i < 5000; i++ {
		fs.Write(makeTestEvent())
	}
	droppedAfter := fs.Dropped()

	if droppedAfter <= droppedBefore {
		t.Error("expected some events to be dropped when channel is full")
	}

	fs.Close()
}

func TestFileSink_CloseFlushes(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	fs.Write(makeTestEvent())
	fs.Write(makeTestEvent())
	fs.Close()

	data, err := os.ReadFile(filepath.Join(dir, "trace-2026-04-10.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Count(string(data), "\n")
	if lines != 2 {
		t.Errorf("expected 2 lines after flush, got %d", lines)
	}
}
