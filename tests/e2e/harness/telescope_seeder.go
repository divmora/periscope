package harness

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// RawTelescopeEntry represents a single row in the source telescope_entries table.
type RawTelescopeEntry struct {
	Sequence             int64     `json:"sequence"`
	UUID                 string    `json:"uuid"`
	BatchID              string    `json:"batch_id"`
	FamilyHash           *string   `json:"family_hash"`
	ShouldDisplayOnIndex bool      `json:"should_display_on_index"`
	Type                 string    `json:"type"`
	Content              string    `json:"content"`
	CreatedAt            time.Time `json:"created_at"`
	Tags                 []string  `json:"tags,omitempty"`
}

// TelescopeTag represents a single row in the source telescope_entries_tags table.
type TelescopeTag struct {
	EntryUUID string `json:"entry_uuid"`
	Tag       string `json:"tag"`
}

// AllKnownEntryTypes lists all 16 supported Laravel Telescope entry types.
var AllKnownEntryTypes = []string{
	"request",
	"query",
	"exception",
	"log",
	"job",
	"command",
	"model",
	"view",
	"event",
	"mail",
	"notification",
	"cache",
	"redis",
	"dump",
	"gate",
	"client_request",
}

// GenerateEntryPayload generates a realistic JSON payload for any known Telescope entry type.
func GenerateEntryPayload(entryType string, seed int64) (string, []string) {
	rng := rand.New(rand.NewSource(seed))

	switch entryType {
	case "request":
		statusCodes := []int{200, 201, 204, 400, 401, 403, 404, 500, 502}
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		uris := []string{"/api/v1/users", "/api/v1/projects", "/api/v1/auth/login", "/dashboard", "/checkout"}
		status := statusCodes[rng.Intn(len(statusCodes))]
		method := methods[rng.Intn(len(methods))]
		uri := uris[rng.Intn(len(uris))]
		duration := float64(rng.Intn(5000)) / 10.0 // 0 to 500 ms
		memory := float64(rng.Intn(640)) / 10.0   // 0 to 64 MB
		userID := fmt.Sprintf("%d", rng.Intn(100)+1)
		userEmail := fmt.Sprintf("user%s@example.com", userID)

		payload := map[string]any{
			"uri":               uri,
			"method":            method,
			"controller_action": "App\\Http\\Controllers\\ApiController@handle",
			"middleware":        []string{"web", "auth:sanctum"},
			"headers": map[string]string{
				"host":       "example.com",
				"user-agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
				"accept":     "application/json",
			},
			"payload": map[string]any{
				"filter": "active",
				"page":   1,
			},
			"response_status": status,
			"duration":        duration,
			"memory":          memory,
			"user": map[string]any{
				"id":    userID,
				"email": userEmail,
				"name":  fmt.Sprintf("User %s", userID),
			},
		}
		data, _ := json.Marshal(payload)
		tags := []string{
			fmt.Sprintf("Auth:%s", userID),
			fmt.Sprintf("Status:%d", status),
			fmt.Sprintf("Method:%s", method),
		}
		return string(data), tags

	case "query":
		tables := []string{"users", "projects", "telescope_entries", "orders", "invoices"}
		table := tables[rng.Intn(len(tables))]
		queryTime := float64(rng.Intn(1500)) / 10.0 // 0 to 150 ms
		slow := queryTime > 100.0
		line := int32(rng.Intn(300) + 10)

		payload := map[string]any{
			"connection": "mysql",
			"bindings":   []any{rng.Intn(1000) + 1, "active"},
			"sql":        fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND status = ? LIMIT 10", table),
			"time":       queryTime,
			"slow":       slow,
			"file":       fmt.Sprintf("/var/www/app/Repositories/%sRepository.php", table),
			"line":       line,
			"hash":       fmt.Sprintf("%x", seed),
		}
		data, _ := json.Marshal(payload)
		tags := []string{
			fmt.Sprintf("Table:%s", table),
			fmt.Sprintf("Slow:%t", slow),
		}
		return string(data), tags

	case "exception":
		classes := []string{
			"Illuminate\\Database\\QueryException",
			"Symfony\\Component\\HttpKernel\\Exception\\NotFoundHttpException",
			"App\\Exceptions\\PaymentFailedException",
			"App\\Exceptions\\UnauthorizedException",
		}
		excClass := classes[rng.Intn(len(classes))]
		line := int32(rng.Intn(500) + 1)
		userID := fmt.Sprintf("%d", rng.Intn(100)+1)

		payload := map[string]any{
			"class":   excClass,
			"file":    "/var/www/app/Services/PaymentService.php",
			"line":    line,
			"message": fmt.Sprintf("Critical failure encountered in payment routine with code %d", seed),
			"trace": []map[string]any{
				{"file": "/var/www/app/Http/Controllers/CheckoutController.php", "line": 45, "function": "process"},
				{"file": "/var/www/vendor/laravel/framework/src/Illuminate/Routing/Controller.php", "line": 54, "function": "callAction"},
			},
			"user": map[string]any{
				"id":    userID,
				"email": fmt.Sprintf("user%s@example.com", userID),
			},
		}
		data, _ := json.Marshal(payload)
		tags := []string{
			fmt.Sprintf("Class:%s", excClass),
			fmt.Sprintf("User:%s", userID),
		}
		return string(data), tags

	case "log":
		levels := []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}
		level := levels[rng.Intn(len(levels))]
		userID := fmt.Sprintf("%d", rng.Intn(50)+1)

		payload := map[string]any{
			"level":   level,
			"message": fmt.Sprintf("Application event logged with severity %s at step %d", level, seed),
			"context": map[string]any{
				"environment": "production",
				"version":     "v2.4.1",
				"memory_peak": "34.5MB",
			},
			"user": map[string]any{
				"id":    userID,
				"email": fmt.Sprintf("admin%s@example.com", userID),
			},
		}
		data, _ := json.Marshal(payload)
		tags := []string{
			fmt.Sprintf("Level:%s", level),
		}
		return string(data), tags

	case "job":
		jobs := []string{
			"App\\Jobs\\SendEmailNotification",
			"App\\Jobs\\GenerateMonthlyInvoice",
			"App\\Jobs\\ProcessImageUpload",
			"App\\Jobs\\SyncTelemetryData",
		}
		jobName := jobs[rng.Intn(len(jobs))]
		statuses := []string{"pending", "processing", "completed", "failed"}
		status := statuses[rng.Intn(len(statuses))]
		tries := int32(rng.Intn(3) + 1)

		payload := map[string]any{
			"name":       jobName,
			"queue":      "high_priority",
			"connection": "redis",
			"status":     status,
			"tries":      tries,
			"timeout":    120,
			"data": map[string]any{
				"entity_id": rng.Intn(10000),
				"dry_run":   false,
			},
		}
		data, _ := json.Marshal(payload)
		tags := []string{
			fmt.Sprintf("Job:%s", jobName),
			fmt.Sprintf("Status:%s", status),
		}
		return string(data), tags

	case "command":
		commands := []string{"migrate", "queue:work", "telemetry:prune", "cache:clear", "config:cache"}
		cmdName := commands[rng.Intn(len(commands))]
		exitCode := int32(0)
		if rng.Intn(10) == 0 {
			exitCode = 1
		}

		payload := map[string]any{
			"command":   cmdName,
			"exit_code": exitCode,
			"arguments": map[string]string{"--force": "true"},
			"options":   map[string]string{"--verbose": "1"},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Command:%s", cmdName)}
		return string(data), tags

	case "model":
		models := []string{"App\\Models\\User", "App\\Models\\Project", "App\\Models\\Invoice"}
		actions := []string{"created", "updated", "deleted", "restored"}
		m := models[rng.Intn(len(models))]
		action := actions[rng.Intn(len(actions))]

		payload := map[string]any{
			"action":  action,
			"model":   m,
			"count":   1,
			"changes": map[string]any{"updated_at": time.Now().UTC().Format(time.RFC3339)},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Model:%s", m), fmt.Sprintf("Action:%s", action)}
		return string(data), tags

	case "view":
		views := []string{"auth.login", "dashboard.overview", "projects.show", "emails.welcome"}
		v := views[rng.Intn(len(views))]

		payload := map[string]any{
			"name": v,
			"path": fmt.Sprintf("resources/views/%s.blade.php", v),
			"data": map[string]any{"title": "Dashboard", "count": 42},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("View:%s", v)}
		return string(data), tags

	case "event":
		events := []string{"App\\Events\\UserRegistered", "App\\Events\\ProjectCreated", "App\\Events\\OrderPlaced"}
		ev := events[rng.Intn(len(events))]

		payload := map[string]any{
			"name":      ev,
			"listeners": []string{"App\\Listeners\\SendWelcomeEmail", "App\\Listeners\\LogActivity"},
			"payload":   map[string]any{"id": seed},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Event:%s", ev)}
		return string(data), tags

	case "mail":
		payload := map[string]any{
			"mailable": "App\\Mail\\InvoiceGeneratedMail",
			"subject":  fmt.Sprintf("Your Monthly Invoice #%d", seed),
			"from":     "billing@periscope.io",
			"to":       "customer@example.com",
			"cc":       "accounts@periscope.io",
		}
		data, _ := json.Marshal(payload)
		tags := []string{"Mailable:InvoiceGeneratedMail", "To:customer@example.com"}
		return string(data), tags

	case "notification":
		payload := map[string]any{
			"notification": "App\\Notifications\\NewProjectAlert",
			"notifiable":   fmt.Sprintf("App\\Models\\User:%d", seed%50+1),
			"channel":      "mail",
			"response":     "250 Message queued",
		}
		data, _ := json.Marshal(payload)
		tags := []string{"Notification:NewProjectAlert", "Channel:mail"}
		return string(data), tags

	case "cache":
		actions := []string{"hit", "miss", "set", "forget"}
		act := actions[rng.Intn(len(actions))]
		key := fmt.Sprintf("cache:project:%d:config", seed%20+1)

		payload := map[string]any{
			"type":       act,
			"key":        key,
			"value":      "{\"status\":\"active\"}",
			"expiration": 3600,
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Cache:%s", act), fmt.Sprintf("Key:%s", key)}
		return string(data), tags

	case "redis":
		commands := []string{"GET", "SET", "HGETALL", "LPUSH", "EXPIRE"}
		cmd := commands[rng.Intn(len(commands))]
		execTime := float64(rng.Intn(100)) / 10.0 // 0 to 10 ms

		payload := map[string]any{
			"connection": "default",
			"command":    fmt.Sprintf("%s session:%d", cmd, seed%100+1),
			"time":       execTime,
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Redis:%s", cmd)}
		return string(data), tags

	case "dump":
		payload := map[string]any{
			"dump": fmt.Sprintf("<pre class=sf-dump>array:2 [\"id\" => %d, \"name\" => \"DumpTest\"]</pre>", seed),
		}
		data, _ := json.Marshal(payload)
		tags := []string{"Dump"}
		return string(data), tags

	case "gate":
		abilities := []string{"view-dashboard", "edit-project", "delete-user", "manage-billing"}
		ab := abilities[rng.Intn(len(abilities))]
		results := []string{"allowed", "denied"}
		res := results[rng.Intn(len(results))]
		userID := fmt.Sprintf("%d", seed%20+1)

		payload := map[string]any{
			"ability":   ab,
			"result":    res,
			"arguments": map[string]any{"project_id": 42},
			"user": map[string]any{
				"id":    userID,
				"email": fmt.Sprintf("user%s@example.com", userID),
			},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("Gate:%s", ab), fmt.Sprintf("Result:%s", res)}
		return string(data), tags

	case "client_request":
		methods := []string{"GET", "POST", "PUT"}
		method := methods[rng.Intn(len(methods))]
		status := 200
		if rng.Intn(5) == 0 {
			status = 500
		}
		duration := float64(rng.Intn(1000)) / 10.0

		payload := map[string]any{
			"method":          method,
			"uri":             "https://api.external-service.com/v1/webhook",
			"response_status": status,
			"duration":        duration,
			"headers":         map[string]string{"authorization": "Bearer secret-token"},
			"payload":         map[string]any{"event": "ping"},
		}
		data, _ := json.Marshal(payload)
		tags := []string{fmt.Sprintf("ClientStatus:%d", status), fmt.Sprintf("ClientMethod:%s", method)}
		return string(data), tags

	default:
		payload := map[string]any{
			"data": fmt.Sprintf("Generic payload for type %s with seed %d", entryType, seed),
		}
		data, _ := json.Marshal(payload)
		return string(data), []string{fmt.Sprintf("Type:%s", entryType)}
	}
}

// GenerateEntriesBatch generates a slice of RawTelescopeEntry and associated TelescopeTag items.
// If types is empty, it round-robins across AllKnownEntryTypes.
func GenerateEntriesBatch(startSeq int64, count int, baseTime time.Time, timeStep time.Duration, types []string) ([]RawTelescopeEntry, []TelescopeTag) {
	if len(types) == 0 {
		types = AllKnownEntryTypes
	}

	entries := make([]RawTelescopeEntry, count)
	var allTags []TelescopeTag

	for i := 0; i < count; i++ {
		seq := startSeq + int64(i)
		entryType := types[i%len(types)]
		entryUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("entry-%d", seq))).String()
		batchUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("batch-%d", seq/100))).String()
		familyHash := fmt.Sprintf("fam-%x", seq%50)
		entryTime := baseTime.Add(time.Duration(i) * timeStep).UTC()

		contentJSON, tags := GenerateEntryPayload(entryType, seq)

		entries[i] = RawTelescopeEntry{
			Sequence:             seq,
			UUID:                 entryUUID,
			BatchID:              batchUUID,
			FamilyHash:           &familyHash,
			ShouldDisplayOnIndex: true,
			Type:                 entryType,
			Content:              contentJSON,
			CreatedAt:            entryTime,
			Tags:                 tags,
		}

		for _, tag := range tags {
			allTags = append(allTags, TelescopeTag{
				EntryUUID: entryUUID,
				Tag:       tag,
			})
		}
	}

	return entries, allTags
}
