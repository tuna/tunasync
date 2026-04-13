package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const LevelNotice = slog.Level(2)

type Logger struct {
	name string
}

type lineHandler struct {
	w           io.Writer
	level       slog.Leveler
	addSource   bool
	withSystemd bool
	attrs       []slog.Attr
	groups      []string
	mu          *sync.Mutex
}

var defaultHandler atomic.Value

func init() {
	InitLogger(false, false, false)
}

func MustGetLogger(name string) *Logger {
	return &Logger{name: name}
}

// InitLogger initializes logging format and level.
func InitLogger(verbose, debug, withSystemd bool) {
	level := LevelNotice
	if debug {
		level = slog.LevelDebug
	} else if verbose {
		level = slog.LevelInfo
	}
	defaultHandler.Store(slog.Handler(newLineHandler(os.Stdout, level, debug, withSystemd)))
}

func (l *Logger) Debug(args ...any) {
	l.log(slog.LevelDebug, normalizeArgs(args...))
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(slog.LevelDebug, formatMessage(format, args...))
}

func (l *Logger) Info(args ...any) {
	l.log(slog.LevelInfo, normalizeArgs(args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(slog.LevelInfo, formatMessage(format, args...))
}

func (l *Logger) Notice(args ...any) {
	l.log(LevelNotice, normalizeArgs(args...))
}

func (l *Logger) Noticef(format string, args ...any) {
	l.log(LevelNotice, formatMessage(format, args...))
}

func (l *Logger) Warning(args ...any) {
	l.log(slog.LevelWarn, normalizeArgs(args...))
}

func (l *Logger) Warningf(format string, args ...any) {
	l.log(slog.LevelWarn, formatMessage(format, args...))
}

func (l *Logger) Error(args ...any) {
	l.log(slog.LevelError, normalizeArgs(args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(slog.LevelError, formatMessage(format, args...))
}

func (l *Logger) Panic(args ...any) {
	msg := normalizeArgs(args...)
	l.log(slog.LevelError, msg)
	panic(msg)
}

func (l *Logger) Panicf(format string, args ...any) {
	msg := formatMessage(format, args...)
	l.log(slog.LevelError, msg)
	panic(msg)
}

func (l *Logger) log(level slog.Level, msg string) {
	handler := currentHandler()
	ctx := context.Background()
	if !handler.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	record := slog.NewRecord(time.Now(), level, msg, pcs[0])
	_ = handler.Handle(ctx, record)
}

func currentHandler() slog.Handler {
	if h, ok := defaultHandler.Load().(slog.Handler); ok {
		return h
	}
	return newLineHandler(os.Stdout, LevelNotice, false, false)
}

func newLineHandler(w io.Writer, level slog.Leveler, addSource, withSystemd bool) *lineHandler {
	return &lineHandler{
		w:           w,
		level:       level,
		addSource:   addSource,
		withSystemd: withSystemd,
		mu:          &sync.Mutex{},
	}
}

func (h *lineHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *lineHandler) Handle(_ context.Context, record slog.Record) error {
	var b strings.Builder

	if h.withSystemd {
		b.WriteString("[")
		b.WriteString(levelLabel(record.Level))
		b.WriteString("] ")
	} else {
		b.WriteString("[")
		b.WriteString(record.Time.Format("06-01-02 15:04:05"))
		b.WriteString("][")
		b.WriteString(levelLabel(record.Level))
		b.WriteString("]")
		if h.addSource {
			if src := shortSource(record.PC); src != "" {
				b.WriteString("[")
				b.WriteString(src)
				b.WriteString("]")
			}
		}
		b.WriteString(" ")
	}

	b.WriteString(record.Message)

	attrs := append([]slog.Attr{}, h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	appendAttrs(&b, h.groups, attrs)

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := *h
	cloned.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cloned
}

func (h *lineHandler) WithGroup(name string) slog.Handler {
	cloned := *h
	cloned.groups = append(append([]string{}, h.groups...), name)
	return &cloned
}

func appendAttrs(b *strings.Builder, groups []string, attrs []slog.Attr) {
	for _, attr := range attrs {
		attr.Value = attr.Value.Resolve()
		if attr.Equal(slog.Attr{}) {
			continue
		}
		key := attr.Key
		if len(groups) > 0 {
			key = strings.Join(append(append([]string{}, groups...), key), ".")
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(attrValue(attr.Value))
	}
}

func attrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return strconv.Quote(v.String())
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return fmt.Sprintf("%v", v.Any())
	default:
		return v.String()
	}
}

func levelLabel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DEBUG"
	case level < LevelNotice:
		return "INFO"
	case level < slog.LevelWarn:
		return "NOTICE"
	case level < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

func shortSource(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.File == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
}

func normalizeArgs(args ...any) string {
	if len(args) == 0 {
		return ""
	}
	if format, ok := args[0].(string); ok && len(args) > 1 && strings.Contains(format, "%") {
		return fmt.Sprintf(format, args[1:]...)
	}
	return fmt.Sprint(args...)
}

func formatMessage(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	if strings.Contains(format, "%") {
		return fmt.Sprintf(format, args...)
	}
	items := make([]any, 0, len(args)+1)
	items = append(items, format)
	items = append(items, args...)
	return fmt.Sprint(items...)
}
