package logger

import (
	"log"
	"log/syslog"
)

func init() {
	// Configure syslog
	// LOG_INFO: Info level
	// LOG_LOCAL0: Local use 0
	sysLog, err := syslog.New(syslog.LOG_INFO|syslog.LOG_LOCAL0, "periscope")
	if err == nil {
		// Replace standard log output with syslog
		log.SetOutput(sysLog)
		log.SetFlags(0) // syslog already adds timestamp, so we can disable standard log flags
	} else {
		log.Println("Notice: Failed to initialize syslog (not supported on this OS?), falling back to standard stdout logging.")
	}
}
