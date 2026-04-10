package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// fileEventJSON is the JSON structure written to log files.
type fileEventJSON struct {
	FlowID    uint64                 `json:"flow_id"`
	Timestamp string                 `json:"timestamp"`
	Span      string                 `json:"span"`
	Event     string                 `json:"event"`
	SpellID   uint32                 `json:"spell_id"`
	SpellName string                 `json:"spell_name"`
	Fields    map[string]interface{} `json:"fields"`
}

// FileSink writes FlowEvents as JSON-lines to daily-rotated log files.
// Writes are non-blocking via a buffered channel; overflow events are dropped.
type FileSink struct {
	dir      string
	ch       chan FlowEvent
	done     chan struct{}
	wg       sync.WaitGroup
	file     *os.File
	date     string // "YYYY-MM-DD"
	dropped  int64
}

// NewFileSink creates a FileSink that writes to dir/trace-YYYY-MM-DD.log.
// The directory is created if it does not exist.
func NewFileSink(dir string) (*FileSink, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
	}

	fs := &FileSink{
		dir:  dir,
		ch:   make(chan FlowEvent, 4096),
		done: make(chan struct{}),
	}

	// Open today's file
	today := time.Now().Format("2006-01-02")
	if err := fs.openFile(today); err != nil {
		return nil, err
	}

	fs.wg.Add(1)
	go fs.loop()

	return fs, nil
}

// Write enqueues the event for async writing. Never blocks beyond channel capacity.
// If the channel is full, the event is dropped and the drop counter incremented.
func (fs *FileSink) Write(e FlowEvent) {
	select {
	case fs.ch <- e:
	default:
		atomic.AddInt64(&fs.dropped, 1)
	}
}

// Dropped returns the number of events dropped due to full channel.
func (fs *FileSink) Dropped() int64 {
	return atomic.LoadInt64(&fs.dropped)
}

// Close flushes remaining events and closes the log file.
func (fs *FileSink) Close() {
	close(fs.done)
	fs.wg.Wait()
	if fs.file != nil {
		fs.file.Close()
	}
}

func (fs *FileSink) loop() {
	defer fs.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-fs.done:
			// Drain remaining
			for {
				select {
				case e := <-fs.ch:
					fs.writeEvent(e)
				default:
					return
				}
			}
		case e := <-fs.ch:
			fs.checkRotation()
			fs.writeEvent(e)
		case <-ticker.C:
			fs.checkRotation()
		}
	}
}

func (fs *FileSink) checkRotation() {
	today := time.Now().Format("2006-01-02")
	if today != fs.date {
		if fs.file != nil {
			fs.file.Close()
		}
		_ = fs.openFile(today)
	}
}

func (fs *FileSink) openFile(date string) error {
	name := filepath.Join(fs.dir, "trace-"+date+".log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", name, err)
	}
	fs.file = f
	fs.date = date
	return nil
}

func (fs *FileSink) writeEvent(e FlowEvent) {
	line := fileEventJSON{
		FlowID:    e.FlowID,
		Timestamp: e.Timestamp.Format(time.RFC3339Nano),
		Span:      e.Span,
		Event:     e.Event,
		SpellID:   e.SpellID,
		SpellName: e.SpellName,
		Fields:    e.Fields,
	}
	data, err := json.Marshal(line)
	if err != nil {
		return
	}
	fs.file.Write(data)
	fs.file.Write([]byte("\n"))
}
