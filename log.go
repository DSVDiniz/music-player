package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Level string

const (
	INFO    Level = "INFO"
	SUCCESS Level = "SUCCESS"
	DEBUG   Level = "DEBUG"
	WARN    Level = "WARN"
	ERROR   Level = "ERROR"
	PANIC   Level = "PANIC"
)
const (
	colorReset     = "\033[0m"
	colorRed       = "\033[31m"
	colorBrightRed = "\033[91m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
	colorBlue      = "\033[34m"
	colorGray      = "\033[90m"
)

var levelColor = map[Level]string{
	INFO:    colorBlue,
	SUCCESS: colorGreen,
	DEBUG:   colorGray,
	WARN:    colorYellow,
	ERROR:   colorRed,
	PANIC:   colorBrightRed,
}

type Logger struct {
	console *log.Logger
	file    *log.Logger
}

func NewLogger(path string) (*Logger, error) {
	if len(path) > 0 {
		f, err := os.OpenFile(
			path,
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return nil, err
		}
		return &Logger{
			console: log.New(os.Stdout, "", 0),
			file:    log.New(f, "", 0),
		}, nil
	}
	return &Logger{
		console: log.New(os.Stdout, "", 0),
		file:    nil,
	}, nil
}

func (lg *Logger) log(level Level, format string, args ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	color := levelColor[level]
	msg := fmt.Sprintf(format, args...)
	consoleLine := fmt.Sprintf(
		"%s[%s] [% -7s] %s%s",
		color, ts, level, msg, colorReset,
	)
	lg.console.Println(consoleLine)

	if lg.file != nil {
		fileLine := fmt.Sprintf(
			"[%s] [% -7s] %s",
			ts, level, msg,
		)
		lg.file.Println(fileLine)
	}
}

func (lg *Logger) Info(f string, a ...any)    { lg.log(INFO, f, a...) }
func (lg *Logger) Success(f string, a ...any) { lg.log(SUCCESS, f, a...) }
func (lg *Logger) Debug(f string, a ...any)   { lg.log(DEBUG, f, a...) }
func (lg *Logger) Warn(f string, a ...any)    { lg.log(WARN, f, a...) }
func (lg *Logger) Error(f string, a ...any)   { lg.log(ERROR, f, a...) }

func (lg *Logger) Panic(f string, a ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	color := levelColor[PANIC]

	msg := fmt.Sprintf(f, a...)
	lg.console.Printf(
		"%s[%s] [% -7s] %s%s\n",
		color, ts, PANIC, msg, colorReset,
	)
	if lg.file != nil {
		lg.file.Printf(
			"[%s] [% -7s] %s\n",
			ts, PANIC, msg,
		)
	}

	panic(msg)
}
