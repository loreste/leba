package loadbalancer

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// CallLog represents SIP call log details
type CallLog struct {
	CallID    string    `json:"call_id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Protocol  string    `json:"protocol"`
}

// Logger handles centralized logging for the load balancer
type Logger struct {
	logFilePath string
	file        *os.File
}

// NewLogger creates a new logger instance
func NewLogger(logFilePath string) (*Logger, error) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		logFilePath: logFilePath,
		file:        file,
	}, nil
}

// LogMessage writes a general message to the log file
func (l *Logger) LogMessage(message string) {
	log.Printf("[General Log] %s", message)

	_, err := l.file.WriteString(time.Now().Format(time.RFC3339) + " - " + message + "\n")
	if err != nil {
		log.Printf("[Error] Failed to write message to log file: %v", err)
	}
}

// LogCall writes SIP call details to the log file
func (l *Logger) LogCall(callLog CallLog) {
	log.Printf("[Call Log] CallID: %s, From: %s, To: %s, Timestamp: %s, Status: %s, Protocol: %s",
		callLog.CallID, callLog.From, callLog.To, callLog.Timestamp.Format(time.RFC3339), callLog.Status, callLog.Protocol)

	jsonLog, err := json.Marshal(callLog)
	if err != nil {
		log.Printf("[Error] Failed to marshal call log: %v", err)
		return
	}

	_, err = l.file.Write(append(jsonLog, '\n'))
	if err != nil {
		log.Printf("[Error] Failed to write call log to log file: %v", err)
	}
}

// Close closes the log file
func (l *Logger) Close() {
	if err := l.file.Close(); err != nil {
		log.Printf("[Error] Failed to close log file: %v", err)
	}
}
