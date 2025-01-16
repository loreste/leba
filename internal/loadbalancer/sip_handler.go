package loadbalancer

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// CallLog represents SIP call log details

// SIPHandler handles SIP-specific logic
type SIPHandler struct {
	logFilePath string
}

// NewSIPHandler creates a new SIPHandler instance
func NewSIPHandler(logFilePath string) *SIPHandler {
	return &SIPHandler{
		logFilePath: logFilePath,
	}
}

// logCall logs SIP call details to a file
func (sh *SIPHandler) logCall(callLog CallLog) {
	log.Printf("[Call Log] CallID: %s, From: %s, To: %s, Timestamp: %s, Status: %s, Protocol: %s",
		callLog.CallID, callLog.From, callLog.To, callLog.Timestamp.Format(time.RFC3339), callLog.Status, callLog.Protocol)

	file, err := os.OpenFile(sh.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[Call Log] Failed to open log file: %v", err)
		return
	}
	defer file.Close()

	jsonLog, err := json.Marshal(callLog)
	if err != nil {
		log.Printf("[Call Log] Failed to marshal call log: %v", err)
		return
	}

	_, err = file.Write(append(jsonLog, '\n'))
	if err != nil {
		log.Printf("[Call Log] Failed to write to log file: %v", err)
	}
}
