package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

type Level int8

// 代表着具体的安全级别
const (
	LevelInfo Level = iota
	LevelError
	LevelFatal
	LevelOff
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// Logger Define a custom Logger type,包括了log entries的写入目标，最低的安全等级和写锁
// 本质上是对io.Writer的一种包装器，最后将日志变为JSON写入io.Writer
type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

// Return a new Logger instance,并没有全部进行赋值
func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
	}
}

// Declare some helper methods for writing log entries at the different levels
// map用于包含你希望在日志entry中的任何属性
func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties)
}

func (l *Logger) PrintError(err error, properties map[string]string) {
	l.print(LevelError, err.Error(), properties)
}

func (l *Logger) PrintFatal(err error, properties map[string]string) {
	l.print(LevelFatal, err.Error(), properties)
	os.Exit(1) //如果是Fatal级别，需要终止程序？
}

// 用于写入日志entry的内部方法
func (l *Logger) print(level Level, message string, properties map[string]string) (int, error) {
	// 如果等级比Logger的最低安全级别要低，不做操作
	if level < l.minLevel {
		return 0, nil
	}

	// Declare an anonymous struct holding the data for log entry
	aux := struct {
		Level      string            `json:"level"`
		Time       string            `json:"time"`
		Message    string            `json:"message"`
		Properties map[string]string `json:"properties,omitempty"`
		Trace      string            `json:"trace,omitempty"`
	}{
		Level:      level.String(), // 如何将日志级别从012转为string
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: properties, // 也没有全部初始化,自定义Error和FATAL才有trace
	}

	// Include a stack trace for entries at the ERROR and FATAL levels
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}

	// Declare a line variable for holding the actual log entry text
	var line []byte

	// Marshal the anonymous struct to JSON and store it in the line
	line, err := json.Marshal(aux)
	if err != nil {
		line = []byte(LevelError.String() + ":unable to marshal log messages:" + err.Error())
	}

	// 防止多个写到目标地址out
	l.mu.Lock()
	defer l.mu.Unlock() // 结束后解锁

	return l.out.Write(append(line, '\n'))
}

// We also implement a Write() method on our logger type so it satisfies the io.Writer interface
// 可以作为任何需要io.Writer类型的地方使用
// Writer接口只有一个Write方法
func (l *Logger) Write(message []byte) (n int, err error) {
	return l.print(LevelError, string(message), nil)
}
