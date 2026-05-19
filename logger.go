package banter

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	logDebug = iota
	logInfo
	logWarn
	logError
	logCritical
)

var levelNames = map[int]string{
	logDebug:    "DEBUG",
	logInfo:     "INFO",
	logWarn:     "WARNING",
	logError:    "ERROR",
	logCritical: "CRITICAL",
}

var levelColours = map[int]string{
	logDebug:    "\x1b[36m",
	logInfo:     "\x1b[34m",
	logWarn:     "\x1b[33m",
	logError:    "\x1b[31m",
	logCritical: "\x1b[41m",
}

const (
	colourReset = "\x1b[0m"
	colourDim   = "\x1b[30;1m"
	colourName  = "\x1b[35m"
)

var (
	rootMu         sync.Mutex
	rootLevel      = logInfo
	rootUseColour  = detectColour()
	rootInitialized bool
)

func detectColour() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func initRoot() {
	rootMu.Lock()
	defer rootMu.Unlock()
	if rootInitialized {
		return
	}
	rootInitialized = true
	debugEnv := strings.ToLower(os.Getenv("BANTERAPI_DEBUG"))
	if debugEnv == "1" || debugEnv == "true" || debugEnv == "yes" {
		rootLevel = logDebug
	}
}

func setLogLevel(level int) {
	rootMu.Lock()
	defer rootMu.Unlock()
	rootLevel = level
}

type Logger struct {
	name string
}

func newLogger(name string) *Logger {
	initRoot()
	if !strings.HasPrefix(name, "banterapi") {
		name = "banterapi." + name
	}
	return &Logger{name: name}
}

func (l *Logger) write(level int, format string, args ...any) {
	rootMu.Lock()
	if level < rootLevel {
		rootMu.Unlock()
		return
	}
	useColour := rootUseColour
	rootMu.Unlock()

	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	levelName := levelNames[level]

	var line string
	if useColour {
		colour := levelColours[level]
		line = fmt.Sprintf("%s%s%s %s%-8s%s %s%s%s %s\n",
			colourDim, ts, colourReset,
			colour, levelName, colourReset,
			colourName, l.name, colourReset,
			msg)
	} else {
		line = fmt.Sprintf("%s [%s] %s: %s\n", ts, l.name, levelName, msg)
	}
	fmt.Fprint(os.Stderr, line)
}

func (l *Logger) Debug(format string, args ...any) { l.write(logDebug, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.write(logInfo, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.write(logWarn, format, args...) }
func (l *Logger) Error(format string, args ...any) { l.write(logError, format, args...) }

func (l *Logger) Panic(format string, args ...any) {
	l.write(logCritical, format, args...)
	os.Exit(1)
}