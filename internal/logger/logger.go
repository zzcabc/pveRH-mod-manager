// Package logger 提供带日志级别的控制台日志工具。
// 基于 Go 标准 log 包，输出到 stderr，通过环境变量 LOG_LEVEL 控制级别。
package logger

import (
	"log"
	"os"
	"strings"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var currentLevel Level = INFO

func init() {
	log.SetFlags(log.LstdFlags)
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		currentLevel = DEBUG
	case "info":
		currentLevel = INFO
	case "warn":
		currentLevel = WARN
	case "error":
		currentLevel = ERROR
	}
}

// SetLevel 设置当前日志级别
func SetLevel(level Level) {
	currentLevel = level
}

// Debug 输出 DEBUG 级别日志
func Debug(v ...interface{}) {
	if currentLevel <= DEBUG {
		log.Println(append([]interface{}{"[DEBUG]"}, v...)...)
	}
}

// Debugf 输出格式化的 DEBUG 级别日志
func Debugf(format string, v ...interface{}) {
	if currentLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info 输出 INFO 级别日志
func Info(v ...interface{}) {
	if currentLevel <= INFO {
		log.Println(append([]interface{}{"[INFO]"}, v...)...)
	}
}

// Infof 输出格式化的 INFO 级别日志
func Infof(format string, v ...interface{}) {
	if currentLevel <= INFO {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warn 输出 WARN 级别日志
func Warn(v ...interface{}) {
	if currentLevel <= WARN {
		log.Println(append([]interface{}{"[WARN]"}, v...)...)
	}
}

// Warnf 输出格式化的 WARN 级别日志
func Warnf(format string, v ...interface{}) {
	if currentLevel <= WARN {
		log.Printf("[WARN] "+format, v...)
	}
}

// Error 输出 ERROR 级别日志
func Error(v ...interface{}) {
	if currentLevel <= ERROR {
		log.Println(append([]interface{}{"[ERROR]"}, v...)...)
	}
}

// Errorf 输出格式化的 ERROR 级别日志
func Errorf(format string, v ...interface{}) {
	if currentLevel <= ERROR {
		log.Printf("[ERROR] "+format, v...)
	}
}
