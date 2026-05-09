// Package logger provides a simple file-based logging system for the application.
// Logs are written to date-stamped files in the user's config directory.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log level constants
const (
	logFileMode = 0644
	logDirMode  = 0755
)

var (
	logFile       *os.File
	logMutex      sync.Mutex
	isInitialized bool
)

// Init initializes the logger with a new log file for today's date.
// It creates the log directory if it doesn't exist.
func Init() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	if isInitialized {
		return nil
	}

	logPath, err := getLogPath()
	if err != nil {
		return fmt.Errorf("failed to get log path: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, logDirMode); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create log file
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFileMode)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	isInitialized = true
	Info("Logger initialized")
	return nil
}

// Close closes the log file handle.
func Close() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	isInitialized = false
}

// Info logs an informational message with timestamp.
// Modified to do nothing so only errors are logged.
func Info(format string, args ...interface{}) {
	// Do nothing
}

// Error logs an error message with timestamp.
func Error(format string, args ...interface{}) {
	log("ERROR", format, args...)
}

// log writes a formatted log message to the log file.
func log(level, format string, args ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Auto-initialize if needed
	if !isInitialized {
		if err := initWithoutLock(); err != nil {
			// Fall back to stderr if logging fails
			fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", level, time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
			return
		}
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

	if logFile != nil {
		logFile.WriteString(logLine)
		logFile.Sync()
	}
}

// initWithoutLock initializes the logger without acquiring the mutex.
// This should only be called when the lock is already held.
func initWithoutLock() error {
	if isInitialized {
		return nil
	}

	logPath, err := getLogPath()
	if err != nil {
		return err
	}

	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, logDirMode); err != nil {
		return err
	}

	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFileMode)
	if err != nil {
		return err
	}

	isInitialized = true
	return nil
}

// getLogPath returns the path to the log file.
func getLogPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".wis-free-v3", "logs", time.Now().Format("2006-01-02")+".log"), nil
}
