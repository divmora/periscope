package collector

import (
	"time"
)

// TelescopeEntry represents a raw record read directly from a Laravel Telescope source database.
type TelescopeEntry struct {
	Sequence             int64     `json:"sequence"`
	UUID                 string    `json:"uuid"`
	BatchID              string    `json:"batch_id"`
	FamilyHash           *string   `json:"family_hash"`
	ShouldDisplayOnIndex bool      `json:"should_display_on_index"`
	Type                 string    `json:"type"`
	Content              string    `json:"content"`
	CreatedAt            time.Time `json:"created_at"`
	Tags                 []string  `json:"tags"`
}

// TelemetryRecord defines the complete typed Parquet schema encoded with ZSTD compression
// and dictionary encoding for high cardinality string fields.
type TelemetryRecord struct {
	// Core Envelope
	Sequence             int64     `parquet:"sequence,delta"`
	UUID                 string    `parquet:"uuid,zstd"`
	BatchID              string    `parquet:"batch_id,zstd"`
	FamilyHash           *string   `parquet:"family_hash,zstd,optional"`
	ShouldDisplayOnIndex bool      `parquet:"should_display_on_index"`
	Source               string    `parquet:"source,dict,zstd"`
	Type                 string    `parquet:"type,dict,zstd"`
	CreatedAt            time.Time `parquet:"created_at,timestamp(microsecond)"`
	Tags                 []string  `parquet:"tags,list,zstd"`

	// Common User Dimensions
	UserID    *string `parquet:"user_id,dict,zstd,optional"`
	UserEmail *string `parquet:"user_email,zstd,optional"`
	UserName  *string `parquet:"user_name,zstd,optional"`

	// HTTP Request Dimensions
	HTTPStatus     *int32   `parquet:"http_status,plain,optional"`
	HTTPMethod     *string  `parquet:"http_method,dict,zstd,optional"`
	HTTPURI        *string  `parquet:"http_uri,zstd,optional"`
	HTTPController *string  `parquet:"http_controller,zstd,optional"`
	DurationMS     *float64 `parquet:"duration_ms,plain,optional"`
	MemoryMB       *float64 `parquet:"memory_mb,plain,optional"`
	ClientIP       *string  `parquet:"client_ip,dict,zstd,optional"`

	// SQL Query Dimensions
	QueryConnection *string  `parquet:"query_connection,dict,zstd,optional"`
	QuerySQL        *string  `parquet:"query_sql,zstd,optional"`
	QueryTimeMS     *float64 `parquet:"query_time_ms,plain,optional"`
	QuerySlow       *bool    `parquet:"query_slow,optional"`
	QueryFile       *string  `parquet:"query_file,zstd,optional"`
	QueryLine       *int32   `parquet:"query_line,plain,optional"`

	// Exception Dimensions
	ExceptionClass   *string `parquet:"exception_class,dict,zstd,optional"`
	ExceptionMessage *string `parquet:"exception_message,zstd,optional"`
	ExceptionFile    *string `parquet:"exception_file,zstd,optional"`
	ExceptionLine    *int32  `parquet:"exception_line,plain,optional"`

	// Job Dimensions
	JobName       *string `parquet:"job_name,dict,zstd,optional"`
	JobQueue      *string `parquet:"job_queue,dict,zstd,optional"`
	JobConnection *string `parquet:"job_connection,dict,zstd,optional"`
	JobStatus     *string `parquet:"job_status,dict,zstd,optional"`
	JobTries      *int32  `parquet:"job_tries,plain,optional"`

	// Command Dimensions
	CommandName     *string `parquet:"command_name,dict,zstd,optional"`
	CommandExitCode *int32  `parquet:"command_exit_code,plain,optional"`

	// Log Dimensions
	LogLevel   *string `parquet:"log_level,dict,zstd,optional"`
	LogMessage *string `parquet:"log_message,zstd,optional"`

	// Model Dimensions
	ModelAction *string `parquet:"model_action,dict,zstd,optional"`
	ModelName   *string `parquet:"model_name,dict,zstd,optional"`
	ModelCount  *int32  `parquet:"model_count,plain,optional"`

	// View Dimensions
	ViewName *string `parquet:"view_name,dict,zstd,optional"`
	ViewPath *string `parquet:"view_path,zstd,optional"`

	// Event Dimensions
	EventName     *string `parquet:"event_name,dict,zstd,optional"`
	ListenerCount *int32  `parquet:"listener_count,plain,optional"`

	// Mail Dimensions
	Mailable    *string `parquet:"mailable,dict,zstd,optional"`
	MailSubject *string `parquet:"mail_subject,zstd,optional"`

	// Notification Dimensions
	NotificationClass *string `parquet:"notification_class,dict,zstd,optional"`
	Notifiable        *string `parquet:"notifiable,zstd,optional"`
	Channel           *string `parquet:"channel,dict,zstd,optional"`

	// Cache Dimensions
	CacheAction *string `parquet:"cache_action,dict,zstd,optional"`
	CacheKey    *string `parquet:"cache_key,zstd,optional"`

	// Redis Dimensions
	RedisConnection *string  `parquet:"redis_connection,dict,zstd,optional"`
	RedisCommand    *string  `parquet:"redis_command,dict,zstd,optional"`
	RedisTimeMS     *float64 `parquet:"redis_time_ms,plain,optional"`

	// Gate Dimensions
	GateAbility *string `parquet:"gate_ability,dict,zstd,optional"`
	GateResult  *string `parquet:"gate_result,dict,zstd,optional"`

	// HTTP Client (client_request) Dimensions
	HTTPClientMethod     *string  `parquet:"http_client_method,dict,zstd,optional"`
	HTTPClientURI        *string  `parquet:"http_client_uri,zstd,optional"`
	HTTPClientStatus     *int32   `parquet:"http_client_status,plain,optional"`
	HTTPClientDurationMS *float64 `parquet:"http_client_duration_ms,plain,optional"`

	// Full Raw Payload
	Content string `parquet:"content,zstd"`
}
