// Copyright 2016 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatedier/golib/log"
)

var (
	TraceLevel = log.TraceLevel
	DebugLevel = log.DebugLevel
	InfoLevel  = log.InfoLevel
	WarnLevel  = log.WarnLevel
	ErrorLevel = log.ErrorLevel
)

const (
	defaultLogBufferSize = 5000
	defaultLogQueryLimit = 300
	maxLogQueryLimit     = 1000
)

type LogEntry struct {
	ID      int64  `json:"id"`
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type LogQueryResult struct {
	Entries    []LogEntry `json:"entries"`
	NextCursor int64      `json:"nextCursor"`
	Truncated  bool       `json:"truncated"`
	Limit      int        `json:"limit"`
}

type logBuffer struct {
	mu       sync.Mutex
	max      int
	nextID   int64
	entries  []LogEntry
	latestID int64
}

type leveledWriter interface {
	WriteLog([]byte, log.Level, time.Time) (n int, err error)
}

type teeWriter struct {
	out io.Writer
}

var (
	Logger    *log.Logger
	bufferLog = newLogBuffer(defaultLogBufferSize)
)

func init() {
	Logger = log.New(
		log.WithCaller(true),
		log.AddCallerSkip(1),
		log.WithLevel(log.InfoLevel),
		log.WithOutput(newTeeWriter(os.Stdout)),
	)
}

func InitLogger(logPath string, levelStr string, maxDays int, disableLogColor bool) {
	options := []log.Option{}
	if logPath == "console" {
		output := io.Writer(os.Stdout)
		if !disableLogColor {
			output = log.NewConsoleWriter(log.ConsoleConfig{
				Colorful: true,
			}, os.Stdout)
		}
		options = append(options, log.WithOutput(newTeeWriter(output)))
	} else {
		writer := log.NewRotateFileWriter(log.RotateFileConfig{
			FileName: logPath,
			Mode:     log.RotateFileModeDaily,
			MaxDays:  maxDays,
		})
		writer.Init()
		options = append(options, log.WithOutput(newTeeWriter(writer)))
	}

	level, err := log.ParseLevel(levelStr)
	if err != nil {
		level = log.InfoLevel
	}
	options = append(options, log.WithLevel(level))
	Logger = Logger.WithOptions(options...)
}

func QueryEntries(cursor int64, limit int, level string) LogQueryResult {
	return bufferLog.query(cursor, limit, level)
}

func ResetBufferForTesting() {
	bufferLog = newLogBuffer(defaultLogBufferSize)
}

func newLogBuffer(max int) *logBuffer {
	return &logBuffer{
		max:     max,
		entries: make([]LogEntry, 0, max),
	}
}

func newTeeWriter(out io.Writer) *teeWriter {
	return &teeWriter{out: out}
}

func (w *teeWriter) Write(p []byte) (n int, err error) {
	bufferLog.add(p, log.InfoLevel, time.Now())
	if w.out == nil {
		return len(p), nil
	}
	return w.out.Write(p)
}

func (w *teeWriter) WriteLog(p []byte, level log.Level, when time.Time) (n int, err error) {
	bufferLog.add(p, level, when)
	if w.out == nil {
		return len(p), nil
	}
	if lw, ok := w.out.(leveledWriter); ok {
		return lw.WriteLog(p, level, when)
	}
	return w.out.Write(p)
}

func (b *logBuffer) add(p []byte, level log.Level, when time.Time) {
	lines := strings.Split(strings.TrimRight(string(p), "\r\n"), "\n")
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, line := range lines {
		message := strings.TrimRight(line, "\r")
		if message == "" {
			continue
		}
		b.nextID++
		b.latestID = b.nextID
		b.entries = append(b.entries, LogEntry{
			ID:      b.nextID,
			Time:    when.Format(time.RFC3339Nano),
			Level:   level.String(),
			Message: message,
		})
		if len(b.entries) > b.max {
			copy(b.entries, b.entries[len(b.entries)-b.max:])
			b.entries = b.entries[:b.max]
		}
	}
}

func (b *logBuffer) query(cursor int64, limit int, level string) LogQueryResult {
	if limit <= 0 {
		limit = defaultLogQueryLimit
	}
	if limit > maxLogQueryLimit {
		limit = maxLogQueryLimit
	}
	level = strings.ToLower(strings.TrimSpace(level))

	b.mu.Lock()
	entries := append([]LogEntry(nil), b.entries...)
	latestID := b.latestID
	b.mu.Unlock()

	filtered := make([]LogEntry, 0, len(entries))
	for _, entry := range entries {
		if cursor > 0 && entry.ID <= cursor {
			continue
		}
		if level != "" && strings.ToLower(entry.Level) != level {
			continue
		}
		filtered = append(filtered, entry)
	}

	truncated := false
	if len(entries) > 0 && cursor > 0 && cursor < entries[0].ID {
		truncated = true
	}
	if len(filtered) > limit {
		truncated = true
		filtered = filtered[len(filtered)-limit:]
	}

	nextCursor := latestID
	if len(filtered) > 0 {
		nextCursor = filtered[len(filtered)-1].ID
	}

	return LogQueryResult{
		Entries:    filtered,
		NextCursor: nextCursor,
		Truncated:  truncated,
		Limit:      limit,
	}
}

func Errorf(format string, v ...any) {
	Logger.Errorf(format, v...)
}

func Warnf(format string, v ...any) {
	Logger.Warnf(format, v...)
}

func Infof(format string, v ...any) {
	Logger.Infof(format, v...)
}

func Debugf(format string, v ...any) {
	Logger.Debugf(format, v...)
}

func Tracef(format string, v ...any) {
	Logger.Tracef(format, v...)
}

func Logf(level log.Level, offset int, format string, v ...any) {
	Logger.Logf(level, offset, format, v...)
}

type WriteLogger struct {
	level  log.Level
	offset int
}

func NewWriteLogger(level log.Level, offset int) *WriteLogger {
	return &WriteLogger{
		level:  level,
		offset: offset,
	}
}

func (w *WriteLogger) Write(p []byte) (n int, err error) {
	Logger.Log(w.level, w.offset, string(bytes.TrimRight(p, "\n")))
	return len(p), nil
}
