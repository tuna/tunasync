package internal

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"
)

type lazyString string

func (l lazyString) LogValue() slog.Value {
	return slog.StringValue(string(l))
}

func withDefaultHandler(t *testing.T, handler slog.Handler) {
	t.Helper()

	prev := currentHandler()
	defaultHandler.Store(handler)
	t.Cleanup(func() {
		defaultHandler.Store(prev)
	})
}

func TestInitLoggerConfiguresHandler(t *testing.T) {
	prev := currentHandler()
	t.Cleanup(func() {
		defaultHandler.Store(prev)
	})

	InitLogger(false, false, true)
	h, ok := currentHandler().(*lineHandler)
	if !ok {
		t.Fatalf("expected *lineHandler, got %T", currentHandler())
	}
	if got := h.level.Level(); got != LevelNotice {
		t.Fatalf("notice level = %v, want %v", got, LevelNotice)
	}
	if !h.withSystemd {
		t.Fatalf("withSystemd = false, want true")
	}
	if h.addSource {
		t.Fatalf("addSource = true, want false")
	}

	InitLogger(true, false, false)
	h = currentHandler().(*lineHandler)
	if got := h.level.Level(); got != slog.LevelInfo {
		t.Fatalf("info level = %v, want %v", got, slog.LevelInfo)
	}
	if h.withSystemd {
		t.Fatalf("withSystemd = true, want false")
	}
	if h.addSource {
		t.Fatalf("addSource = true, want false")
	}

	InitLogger(false, true, false)
	h = currentHandler().(*lineHandler)
	if got := h.level.Level(); got != slog.LevelDebug {
		t.Fatalf("debug level = %v, want %v", got, slog.LevelDebug)
	}
	if !h.addSource {
		t.Fatalf("addSource = false, want true")
	}
}

func TestLoggerMethodsWriteExpectedLevels(t *testing.T) {
	var buf bytes.Buffer
	withDefaultHandler(t, newLineHandler(&buf, slog.LevelDebug, false, false))

	logger := MustGetLogger("unit")
	if logger.name != "unit" {
		t.Fatalf("logger name = %q, want %q", logger.name, "unit")
	}

	debugFormat := "debug %s"
	logger.Debug(debugFormat, "one")
	logger.Debugf("debugf %d", 2)
	logger.Info("info")
	logger.Infof("infof %d", 3)
	logger.Notice("notice")
	logger.Noticef("noticef %d", 4)
	logger.Warning("warning")
	logger.Warningf("warningf %d", 5)
	logger.Error("error")
	logger.Errorf("errorf %d", 6)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 10 {
		t.Fatalf("line count = %d, want 10\n%s", len(lines), buf.String())
	}

	wants := []string{
		"[DEBUG] debug one",
		"[DEBUG] debugf 2",
		"[INFO] info",
		"[INFO] infof 3",
		"[NOTICE] notice",
		"[NOTICE] noticef 4",
		"[WARN] warning",
		"[WARN] warningf 5",
		"[ERROR] error",
		"[ERROR] errorf 6",
	}
	for i, want := range wants {
		if !strings.Contains(lines[i], want) {
			t.Fatalf("line %d = %q, want substring %q", i, lines[i], want)
		}
	}
}

func TestLoggerRespectsLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	withDefaultHandler(t, newLineHandler(&buf, slog.LevelInfo, false, false))

	logger := MustGetLogger("unit")
	logger.Debug("hidden")
	logger.Info("visible")

	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatalf("unexpected debug output: %q", out)
	}
	if !strings.Contains(out, "[INFO] visible") {
		t.Fatalf("missing info output: %q", out)
	}
}

func TestLoggerPanicMethods(t *testing.T) {
	var buf bytes.Buffer
	withDefaultHandler(t, newLineHandler(&buf, slog.LevelDebug, false, false))

	logger := MustGetLogger("panic")

	assertPanic := func(name, want string, fn func()) {
		t.Helper()
		defer func() {
			got := recover()
			if got != want {
				t.Fatalf("%s panic = %v, want %q", name, got, want)
			}
		}()
		fn()
	}

	assertPanic("Panic", "boom", func() {
		logger.Panic("boom")
	})
	if !strings.Contains(buf.String(), "[ERROR] boom") {
		t.Fatalf("panic output missing: %q", buf.String())
	}

	buf.Reset()
	assertPanic("Panicf", "boom 2", func() {
		logger.Panicf("boom %d", 2)
	})
	if !strings.Contains(buf.String(), "[ERROR] boom 2") {
		t.Fatalf("panicf output missing: %q", buf.String())
	}
}

func TestLineHandlerFormatsAttrsAndSource(t *testing.T) {
	var buf bytes.Buffer

	handler := newLineHandler(&buf, slog.LevelDebug, true, false).
		WithGroup("grp").
		WithAttrs([]slog.Attr{
			slog.String("prefix", "yes"),
			{},
		})

	pc, _, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	record := slog.NewRecord(
		time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
		LevelNotice,
		"hello",
		pc,
	)
	record.AddAttrs(
		slog.String("name", "value"),
		slog.Bool("ok", true),
		slog.Int64("n", -3),
		slog.Uint64("u", 4),
		slog.Float64("f", 1.5),
		slog.Duration("d", 2*time.Second),
		slog.Time("when", time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)),
		slog.Any("obj", struct{ X int }{X: 7}),
		slog.Any("lazy", lazyString("resolved")),
	)

	if !handler.Enabled(context.Background(), LevelNotice) {
		t.Fatal("handler should be enabled for notice")
	}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"[24-01-02 03:04:05][NOTICE][logger_test.go:",
		" hello",
		"grp.prefix=\"yes\"",
		"grp.name=\"value\"",
		"grp.ok=true",
		"grp.n=-3",
		"grp.u=4",
		"grp.f=1.5",
		"grp.d=2s",
		"grp.when=2024-01-02T03:04:05Z",
		"grp.obj={7}",
		"grp.lazy=\"resolved\"",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want substring %q", out, want)
		}
	}
}

func TestLineHandlerSystemdFormat(t *testing.T) {
	var buf bytes.Buffer
	handler := newLineHandler(&buf, slog.LevelDebug, false, true)

	record := slog.NewRecord(
		time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
		slog.LevelWarn,
		"systemd",
		0,
	)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	out := buf.String()
	if got, want := out, "[WARN] systemd\n"; got != want {
		t.Fatalf("systemd output = %q, want %q", got, want)
	}
}

func TestLoggerHelpers(t *testing.T) {
	if got := normalizeArgs(); got != "" {
		t.Fatalf("normalizeArgs() = %q, want empty", got)
	}
	format := "value=%d"
	if got := normalizeArgs(format, 4); got != "value=4" {
		t.Fatalf("normalizeArgs(format) = %q, want %q", got, "value=4")
	}
	if got := normalizeArgs("value", 4); got != "value4" {
		t.Fatalf("normalizeArgs(sprint) = %q, want %q", got, "value4")
	}

	if got := formatMessage("plain"); got != "plain" {
		t.Fatalf("formatMessage(no args) = %q, want %q", got, "plain")
	}
	if got := formatMessage(format, 4); got != "value=4" {
		t.Fatalf("formatMessage(format) = %q, want %q", got, "value=4")
	}
	plain := "plain"
	if got := formatMessage(plain, 4); got != "plain4" {
		t.Fatalf("formatMessage(sprint) = %q, want %q", got, "plain4")
	}

	if got := levelLabel(slog.LevelDebug); got != "DEBUG" {
		t.Fatalf("levelLabel(debug) = %q, want DEBUG", got)
	}
	if got := levelLabel(slog.LevelInfo); got != "INFO" {
		t.Fatalf("levelLabel(info) = %q, want INFO", got)
	}
	if got := levelLabel(LevelNotice); got != "NOTICE" {
		t.Fatalf("levelLabel(notice) = %q, want NOTICE", got)
	}
	if got := levelLabel(slog.LevelWarn); got != "WARN" {
		t.Fatalf("levelLabel(warn) = %q, want WARN", got)
	}
	if got := levelLabel(slog.LevelError); got != "ERROR" {
		t.Fatalf("levelLabel(error) = %q, want ERROR", got)
	}

	if got := shortSource(0); got != "" {
		t.Fatalf("shortSource(0) = %q, want empty", got)
	}
	pc, _, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	if got := shortSource(pc); !strings.Contains(got, "logger_test.go:") {
		t.Fatalf("shortSource(pc) = %q, want logger_test.go:*", got)
	}

	if got := attrValue(slog.StringValue("value")); got != "\"value\"" {
		t.Fatalf("attrValue(string) = %q, want %q", got, "\"value\"")
	}
	if got := attrValue(slog.BoolValue(true)); got != "true" {
		t.Fatalf("attrValue(bool) = %q, want true", got)
	}
	if got := attrValue(slog.Int64Value(-3)); got != "-3" {
		t.Fatalf("attrValue(int64) = %q, want -3", got)
	}
	if got := attrValue(slog.Uint64Value(4)); got != "4" {
		t.Fatalf("attrValue(uint64) = %q, want 4", got)
	}
	if got := attrValue(slog.Float64Value(1.5)); got != "1.5" {
		t.Fatalf("attrValue(float64) = %q, want 1.5", got)
	}
	if got := attrValue(slog.DurationValue(2 * time.Second)); got != "2s" {
		t.Fatalf("attrValue(duration) = %q, want 2s", got)
	}
	if got := attrValue(slog.TimeValue(time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC))); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("attrValue(time) = %q, want RFC3339", got)
	}
	if got := attrValue(slog.AnyValue(struct{ X int }{X: 7})); got != "{7}" {
		t.Fatalf("attrValue(any) = %q, want %q", got, "{7}")
	}
	if got := attrValue(slog.GroupValue(slog.Int("x", 1))); !strings.Contains(got, "x=1") {
		t.Fatalf("attrValue(group) = %q, want substring %q", got, "x=1")
	}
}
