// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type LogLevel uint8

const TRACE LogLevel = 0
const DEBUG LogLevel = 1
const INFO LogLevel = 2
const ERROR LogLevel = 3

func (ll LogLevel) String() string {
	switch ll {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case ERROR:
		return "ERROR"
	default:
		return fmt.Sprintf("UNKNOWN(%v)", uint8(ll))
	}

}

const (
	red    = 31
	yellow = 33
	blue   = 36

	syslogDebug = 7
	syslogInfo  = 6
	syslogWarn  = 4
	syslogError = 3
)

type LoggerOptions struct {
	Level                  LogLevel
	Syslog                 bool
	AppInstance            string
	XmonAddr               string      // address to xmon endpoint, empty string for no xmon
	HardwareEventServerURL string      // URL of the server you want to send hardware events OR empty for no logging
	AppType                XmonAppType // only used for xmon
	PrintQuietAlerts       bool        // whether to print alerts in quiet period
}

type Logger struct {
	level     LogLevel
	hasColors bool
	syslog    bool
	xmon      *Xmon
	heClient  *HardwareEventClient
	mu        sync.Mutex
	bufPool   sync.Pool
	out       io.Writer
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fd := int(f.Fd())
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

func (log *Logger) formatLog(level LogLevel, time time.Time, file string, line int, format string, v ...any) {
	var levelColor int
	var syslogPrio int
	switch level {
	case DEBUG, TRACE:
		levelColor = 0 // reset
		syslogPrio = syslogDebug
	case ERROR:
		levelColor = red
		syslogPrio = syslogError
	case INFO:
		levelColor = blue
		syslogPrio = syslogInfo
	default:
		panic(fmt.Errorf("bad loglevel %v", level))
	}

	cs := ""
	ce := ""
	if log.hasColors {
		cs = fmt.Sprintf("\x1b[%dm", levelColor)
		ce = "\x1b[0m"
	}

	buf := log.bufPool.Get().(*bytes.Buffer)
	if log.syslog {
		fmt.Fprintf(buf, "<%d>", syslogPrio)
		fmt.Fprintf(buf, format, v...)
		fmt.Fprintln(buf)
	} else {
		fmt.Fprintf(buf, "%-26s %s:%d [%s%s%s] ", time.Format("2006-01-02T15:04:05.999999"), file, line, cs, level.String(), ce)
		fmt.Fprintf(buf, format, v...)
		fmt.Fprintln(buf)
	}

	log.mu.Lock()
	bytes := buf.Bytes()
	for written := 0; written < len(bytes); {
		w, err := log.out.Write(bytes[written:])
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				// we've already torn down the logging system
				break
			} else {
				log.mu.Unlock()
				panic(fmt.Errorf("could not log: %v", err))
			}
		}
		written += w
	}
	log.mu.Unlock()

	buf.Reset()
	log.bufPool.Put(buf)
}

func NewLogger(
	out *os.File,
	options *LoggerOptions,
) *Logger {
	logger := &Logger{
		level:     options.Level,
		hasColors: isTerminal(out),
		syslog:    options.Syslog,
		bufPool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer([]byte{})
			},
		},
		out: out,
	}

	xmonConfig := XmonConfig{
		PrintQuietAlerts: options.PrintQuietAlerts,
	}
	if options.XmonAddr != "" {
		if options.AppInstance == "" {
			panic(fmt.Errorf("empty app instance"))
		}
		xmonConfig.Addr = options.XmonAddr
		xmonConfig.AppInstance = options.AppInstance
		xmonConfig.AppType = options.AppType
	} else {
		xmonConfig.OnlyLogging = true
	}
	if options.HardwareEventServerURL != "" {
		hec := NewHardwareEventClient(options.HardwareEventServerURL)
		logger.heClient = &hec
	}
	var err error
	logger.xmon, err = NewXmon(logger, &xmonConfig)
	if err != nil {
		panic(err)
	}

	return logger
}

func (l *Logger) Level() LogLevel {
	return l.level
}

func (l *Logger) shouldLog(level LogLevel) bool {
	return level >= l.level
}

func (l *Logger) Log(level LogLevel, format string, v ...any) {
	l.LogStack(1, level, format, v...)
}

func getFileLine(calldepth int) (string, int) {
	// get file
	_, file, line, ok := runtime.Caller(1 + calldepth)
	if !ok {
		file = "???"
		line = 0
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short

	return file, line
}

func (l *Logger) LogLocation(level LogLevel, file string, line int, format string, v ...any) {
	if l.shouldLog(level) {
		l.formatLog(level, time.Now(), file, line, format, v...)
	}
}

func (l *Logger) LogStack(calldepth int, level LogLevel, format string, v ...any) {
	if l.shouldLog(level) {
		file, line := getFileLine(1 + calldepth)
		l.formatLog(level, time.Now(), file, line, format, v...)
	}
}

func (l *Logger) Trace(format string, v ...any) {
	l.LogStack(1, TRACE, format, v...)
}

func (l *Logger) Debug(format string, v ...any) {
	l.LogStack(1, DEBUG, format, v...)
}

func (l *Logger) DebugStack(calldepth int, format string, v ...any) {
	l.LogStack(1+calldepth, DEBUG, format, v...)
}

func (l *Logger) Info(format string, v ...any) {
	l.LogStack(1, INFO, format, v...)
}

func (l *Logger) InfoStack(calldepth int, format string, v ...any) {
	l.LogStack(1+calldepth, INFO, format, v...)
}

// There should be very few times where you want an error log but not an alert.
func (l *Logger) ErrorNoAlert(format string, v ...any) {
	l.LogStack(1, ERROR, format, v...)
}

func (l *Logger) NewNCAlert(quietTime time.Duration) *XmonNCAlert {
	return l.xmon.NewNCAlert(quietTime)
}

func (l *Logger) RaiseAlertStack(appType XmonAppType, calldepth int, format string, v ...any) {
	l.xmon.RaiseStack(l, l.xmon, appType, 1+calldepth, format, v...)
}

func (l *Logger) RaiseAlertAppType(appType XmonAppType, format string, v ...any) {
	l.RaiseAlertStack(appType, 1, format, v...)
}

func (l *Logger) RaiseAlert(format string, v ...any) {
	l.RaiseAlertStack("", 1, format, v...)
}

func (l *Logger) RaiseNCStack(alert *XmonNCAlert, calldepth int, format string, v ...any) {
	alert.RaiseStack(l, l.xmon, 1+calldepth, format, v...)
}

func (l *Logger) RaiseNC(alert *XmonNCAlert, format string, v ...any) {
	l.RaiseNCStack(alert, 1, format, v...)
}

func (l *Logger) ClearNC(alert *XmonNCAlert) {
	alert.Clear(l, l.xmon)
}

// A simple struct to create a json representation of what has gone wrong in a block service.
// Standardized in json to facilitate processing this data in bulk.
type blockServiceErrorMessage struct {
	BlockServiceId string `json:"blockServiceId"`
	ErrorMessage   string `json:"error"`
}

// Asynchronously sends a hardware event to the server or simply logs
// if hardware event logging is not enabled.
func (l *Logger) RaiseHardwareEvent(hostname string, blockServiceID string, msg string) {
	if l.heClient == nil {
		l.Log(syslogError, "Hardware event on %s for block service %s: %s", hostname, blockServiceID, msg)
		return
	}
	f := func() {
		errMsg := blockServiceErrorMessage{
			BlockServiceId: blockServiceID,
			ErrorMessage:   msg,
		}
		msgData, err := json.Marshal(errMsg)
		if err != nil {
			// TODO(nchapma): Stop raising an alert here as soon as we're reasonably confident
			// that this is working.
			l.RaiseAlert("NCHAPMA: Failed to convert hardware event error message to JSON: %V")
			return
		}
		evt := HardwareEvent{
			Hostname:  hostname,
			Timestamp: time.Now(),
			Component: DiskComponent,
			Location:  "EggsFS",
			Message:   string(msgData),
		}
		err = l.heClient.SendHardwareEvent(evt)
		if err != nil {
			// TODO(nchapma): Instead of immediately alerting here, there should really be some kind of
			// retry logic that just queues up requests and only starts to alert if there are
			// more than N events in the queue that can't go out.
			l.RaiseAlert("NCHAPMA: Failed to send hardware event to server: %v", err)
		}
	}
	go f()

}

type loggerSink struct {
	logger *Logger
	level  LogLevel
	buf    *bytes.Buffer
}

func (sink *loggerSink) Write(p []byte) (int, error) {
	if !sink.logger.shouldLog(sink.level) {
		return len(p), nil
	}
	lineBegin := 0
	lineEnd := sink.buf.Len()
	sink.buf.Write(p)
	bytes := sink.buf.Bytes()
	for lineEnd < len(bytes) {
		if bytes[lineEnd] == '\n' {
			sink.logger.LogStack(2, sink.level, "%s", string(bytes[lineBegin:lineEnd]))
			lineBegin = lineEnd + 1
		}
		lineEnd++
	}
	sink.buf.Next(lineBegin)
	return len(p), nil
}

func (l *Logger) Sink(level LogLevel) io.Writer {
	return &loggerSink{
		logger: l,
		level:  level,
		buf:    bytes.NewBuffer([]byte{}),
	}
}
