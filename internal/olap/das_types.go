package olap

import "time"

// DASRecord represents a normalized database activity stream record from AWS Aurora DAS.
type DASRecord struct {
	ID              string    `json:"id"`
	EventTime       time.Time `json:"event_time"`
	DatabaseName    string    `json:"database_name"`
	DBUser          string    `json:"db_user"`
	ClientIP        string    `json:"client_ip"`
	ClientApp       string    `json:"client_app"`
	Command         string    `json:"command"`
	CommandText     string    `json:"command_text"`
	ExecutionTimeMS float64   `json:"execution_time_ms"`
	Status          string    `json:"status"` // 'success', 'failure'
	ErrorMessage    string    `json:"error_message,omitempty"`
	PID             int64     `json:"pid,omitempty"`
}

// DASFilter defines search and pagination options for DAS log queries.
type DASFilter struct {
	Search    string `json:"search"`
	DBUser    string `json:"db_user"`
	ClientIP  string `json:"client_ip"`
	Database  string `json:"database"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

// DASStats represents aggregate KPI metrics for DAS activity.
type DASStats struct {
	TotalQueries  int64            `json:"total_queries"`
	TotalErrors   int64            `json:"total_errors"`
	AvgDurationMS float64          `json:"avg_duration_ms"`
	ActiveUsers   int64            `json:"active_users"`
	CommandCounts map[string]int64 `json:"command_counts"`
	Engine        string           `json:"engine"` // "direct" or "athena"
}
