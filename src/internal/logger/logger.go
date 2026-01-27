package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	fileLogger  *log.Logger
	logFile     *os.File
	initialized bool
	consoleMu   sync.Mutex
)

// helloq InitLogger 初始化文件日志
func InitLogger() error {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("scan_%s.log", timestamp))

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	logFile = f

	fileLogger = log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	initialized = true

	fmt.Printf("📝 Log file created: %s\n", logPath)
	return nil
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

func InfoFileOnly(format string, v ...interface{}) {
	if !initialized {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		msg += "\n"
	}
	fileLogger.Output(2, "[INFO] "+msg)
}

func Info(format string, v ...interface{}) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if !initialized {
		fmt.Printf("[INFO] "+format+"\n", v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		msg += "\n"
	}
	fileLogger.Output(2, "[INFO] "+msg)
	fmt.Print("[INFO] " + msg)
}

func Debug(format string, v ...interface{}) {
	if !initialized {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		msg += "\n"
	}
	fileLogger.Output(2, "[DEBUG] "+msg)
}

func Error(format string, v ...interface{}) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if !initialized {
		fmt.Printf("[ERROR] "+format+"\n", v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		msg += "\n"
	}
	fileLogger.Output(2, "[ERROR] "+msg)
	fmt.Print("[ERROR] " + msg)
}

func Warn(format string, v ...interface{}) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if !initialized {
		fmt.Printf("[WARN] "+format+"\n", v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		msg += "\n"
	}
	fileLogger.Output(2, "[WARN] "+msg)
	fmt.Print("[WARN] " + msg)
}

func GetLogWriter() io.Writer {
	return logFile
}
