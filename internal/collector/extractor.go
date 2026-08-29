package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ExtractRecord transforms a raw TelescopeEntry into a typed TelemetryRecord
// with typed dimensional fields extracted from the JSON content payload.
// If the content is malformed or invalid JSON, it gracefully falls back
// to storing the raw content without panicking and leaves dimensions nil.
func ExtractRecord(entry TelescopeEntry) TelemetryRecord {
	rec := TelemetryRecord{
		Sequence:             entry.Sequence,
		UUID:                 entry.UUID,
		BatchID:              entry.BatchID,
		FamilyHash:           entry.FamilyHash,
		ShouldDisplayOnIndex: entry.ShouldDisplayOnIndex,
		Source:               DefaultSourceLaravelTelescope,
		Type:                 entry.Type,
		CreatedAt:            entry.CreatedAt.UTC(),
		Tags:                 entry.Tags,
		Content:              entry.Content,
	}

	if rec.Tags == nil {
		rec.Tags = []string{}
	}

	if entry.Content == "" {
		return rec
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(entry.Content), &data); err != nil {
		// Graceful fallback on invalid JSON: retain raw content with nil dimensions
		return rec
	}

	// Extract global user dimensions if present in content
	extractUserDimensions(&rec, data)

	// Extract type-specific dimensions
	switch entry.Type {
	case "request":
		rec.HTTPURI = getStringPtr(data, "uri")
		rec.HTTPMethod = getStringPtr(data, "method")
		rec.HTTPController = getStringPtr(data, "controller_action", "controller", "action")
		rec.HTTPStatus = getInt32Ptr(data, "response_status", "status")
		rec.DurationMS = getFloat64Ptr(data, "duration")
		rec.MemoryMB = getFloat64Ptr(data, "memory")
		rec.ClientIP = getStringPtr(data, "ip", "client_ip")

	case "query":
		rec.QueryConnection = getStringPtr(data, "connection")
		rec.QuerySQL = getStringPtr(data, "sql")
		rec.QueryTimeMS = getFloat64Ptr(data, "time")
		rec.QuerySlow = getBoolPtr(data, "slow")
		rec.QueryFile = getStringPtr(data, "file")
		rec.QueryLine = getInt32Ptr(data, "line")

	case "exception":
		rec.ExceptionClass = getStringPtr(data, "class")
		rec.ExceptionMessage = getStringPtr(data, "message")
		rec.ExceptionFile = getStringPtr(data, "file")
		rec.ExceptionLine = getInt32Ptr(data, "line")

	case "log":
		rec.LogLevel = getStringPtr(data, "level")
		rec.LogMessage = getStringPtr(data, "message")

	case "job":
		rec.JobName = getStringPtr(data, "name")
		rec.JobQueue = getStringPtr(data, "queue")
		rec.JobConnection = getStringPtr(data, "connection")
		rec.JobStatus = getStringPtr(data, "status")
		rec.JobTries = getInt32Ptr(data, "tries")

	case "command":
		rec.CommandName = getStringPtr(data, "command", "name")
		rec.CommandExitCode = getInt32Ptr(data, "exit_code", "status")

	case "model":
		rec.ModelAction = getStringPtr(data, "action")
		rec.ModelName = getStringPtr(data, "model")
		rec.ModelCount = getInt32Ptr(data, "count")

	case "view":
		rec.ViewName = getStringPtr(data, "name")
		rec.ViewPath = getStringPtr(data, "path")

	case "event":
		rec.EventName = getStringPtr(data, "name")
		if listeners, ok := data["listeners"]; ok {
			switch l := listeners.(type) {
			case []interface{}:
				count := int32(len(l))
				rec.ListenerCount = &count
			case float64:
				count := int32(l)
				rec.ListenerCount = &count
			case int:
				count := int32(l)
				rec.ListenerCount = &count
			}
		}

	case "mail":
		rec.Mailable = getStringPtr(data, "mailable")
		rec.MailSubject = getStringPtr(data, "subject")

	case "notification":
		rec.NotificationClass = getStringPtr(data, "notification", "class")
		rec.Notifiable = getStringPtr(data, "notifiable")
		rec.Channel = getStringPtr(data, "channel")

	case "cache":
		rec.CacheAction = getStringPtr(data, "type", "action")
		rec.CacheKey = getStringPtr(data, "key")

	case "redis":
		rec.RedisConnection = getStringPtr(data, "connection")
		rec.RedisCommand = getStringPtr(data, "command")
		rec.RedisTimeMS = getFloat64Ptr(data, "time")

	case "gate":
		rec.GateAbility = getStringPtr(data, "ability")
		rec.GateResult = getStringPtr(data, "result")

	case "client_request":
		rec.HTTPClientMethod = getStringPtr(data, "method")
		rec.HTTPClientURI = getStringPtr(data, "uri")
		rec.HTTPClientStatus = getInt32Ptr(data, "response_status", "status")
		rec.HTTPClientDurationMS = getFloat64Ptr(data, "duration")
	}

	return rec
}

func extractUserDimensions(rec *TelemetryRecord, data map[string]interface{}) {
	rawUser, exists := data["user"]
	if !exists || rawUser == nil {
		return
	}

	switch u := rawUser.(type) {
	case map[string]interface{}:
		if id, ok := u["id"]; ok && id != nil {
			idStr := fmt.Sprint(id)
			if idStr != "" {
				rec.UserID = &idStr
			}
		}
		if email, ok := u["email"]; ok && email != nil {
			if emailStr := fmt.Sprint(email); emailStr != "" {
				rec.UserEmail = &emailStr
			}
		}
		if name, ok := u["name"]; ok && name != nil {
			if nameStr := fmt.Sprint(name); nameStr != "" {
				rec.UserName = &nameStr
			}
		}
	}
}

func getStringPtr(data map[string]interface{}, keys ...string) *string {
	for _, key := range keys {
		val, exists := data[key]
		if !exists || val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			if v != "" {
				return &v
			}
		default:
			str := fmt.Sprint(v)
			if str != "" && str != "<nil>" {
				return &str
			}
		}
	}
	return nil
}

func getInt32Ptr(data map[string]interface{}, keys ...string) *int32 {
	for _, key := range keys {
		val, exists := data[key]
		if !exists || val == nil {
			continue
		}
		switch v := val.(type) {
		case float64:
			i := int32(v)
			return &i
		case float32:
			i := int32(v)
			return &i
		case int:
			i := int32(v)
			return &i
		case int64:
			i := int32(v)
			return &i
		case int32:
			return &v
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
				i := int32(parsed)
				return &i
			}
		}
	}
	return nil
}

func getFloat64Ptr(data map[string]interface{}, keys ...string) *float64 {
	for _, key := range keys {
		val, exists := data[key]
		if !exists || val == nil {
			continue
		}
		switch v := val.(type) {
		case float64:
			return &v
		case float32:
			f := float64(v)
			return &f
		case int:
			f := float64(v)
			return &f
		case int64:
			f := float64(v)
			return &f
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func getBoolPtr(data map[string]interface{}, keys ...string) *bool {
	for _, key := range keys {
		val, exists := data[key]
		if !exists || val == nil {
			continue
		}
		switch v := val.(type) {
		case bool:
			return &v
		case int:
			b := v != 0
			return &b
		case float64:
			b := v != 0
			return &b
		case string:
			if parsed, err := strconv.ParseBool(v); err == nil {
				return &parsed
			}
		}
	}
	return nil
}
