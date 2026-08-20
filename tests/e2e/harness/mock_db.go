package harness

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/divmora/periscope/internal/crypto"
	"github.com/divmora/periscope/internal/store"
)

// MockTelescopeDatabase provides an in-memory representation of a Laravel Telescope database
// supporting both MySQL and PostgreSQL dialect behaviors.
type MockTelescopeDatabase struct {
	mu        sync.RWMutex
	dialect   string // "mysql" or "postgres"
	entries   map[int64]RawTelescopeEntry
	tags      map[string][]string // entry_uuid -> []tag
	failQuery bool
	failPrune bool
}

// NewMockTelescopeDatabase creates a new mock database with the specified dialect.
func NewMockTelescopeDatabase(dialect string) *MockTelescopeDatabase {
	return &MockTelescopeDatabase{
		dialect: dialect,
		entries: make(map[int64]RawTelescopeEntry),
		tags:    make(map[string][]string),
	}
}

// Dialect returns the database dialect ("mysql" or "postgres").
func (db *MockTelescopeDatabase) Dialect() string {
	return db.dialect
}

// SetFailures allows injecting simulated database failures.
func (db *MockTelescopeDatabase) SetFailures(failQuery, failPrune bool) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.failQuery = failQuery
	db.failPrune = failPrune
}

// Seed populates the database with entries and tags.
func (db *MockTelescopeDatabase) Seed(entries []RawTelescopeEntry, tags []TelescopeTag) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, e := range entries {
		db.entries[e.Sequence] = e
	}

	for _, t := range tags {
		db.tags[t.EntryUUID] = append(db.tags[t.EntryUUID], t.Tag)
	}
}

// DrainEntries simulates the incremental SELECT query:
// `WHERE sequence > lastSequence (AND type = entryType) ORDER BY sequence ASC LIMIT limit`.
func (db *MockTelescopeDatabase) DrainEntries(entryType string, lastSequence int64, limit int) ([]RawTelescopeEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.failQuery {
		return nil, errors.New("simulated source database connection error")
	}

	var all []RawTelescopeEntry
	for _, e := range db.entries {
		if e.Sequence > lastSequence {
			if entryType == "" || e.Type == entryType {
				all = append(all, e)
			}
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Sequence < all[j].Sequence
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	// Attach tags in memory
	result := make([]RawTelescopeEntry, len(all))
	for i, e := range all {
		copied := e
		if t, ok := db.tags[e.UUID]; ok {
			copied.Tags = make([]string, len(t))
			copy(copied.Tags, t)
		} else {
			copied.Tags = []string{}
		}
		result[i] = copied
	}

	return result, nil
}

// GetTagsForUUIDs simulates `SELECT entry_uuid, tag FROM telescope_entries_tags WHERE entry_uuid IN (...)`.
func (db *MockTelescopeDatabase) GetTagsForUUIDs(uuids []string) (map[string][]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.failQuery {
		return nil, errors.New("simulated tag query error")
	}

	result := make(map[string][]string)
	uuidSet := make(map[string]struct{}, len(uuids))
	for _, u := range uuids {
		uuidSet[u] = struct{}{}
	}

	for u, tagList := range db.tags {
		if _, ok := uuidSet[u]; ok {
			copied := make([]string, len(tagList))
			copy(copied, tagList)
			result[u] = copied
		}
	}

	return result, nil
}

// Prune simulates deleting entries where sequence <= maxSequence (and optional type match).
func (db *MockTelescopeDatabase) Prune(entryType string, maxSequence int64) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.failPrune {
		return 0, errors.New("simulated source pruning lock timeout")
	}

	var deletedCount int64
	var deletedUUIDs []string

	for seq, e := range db.entries {
		if seq <= maxSequence {
			if entryType == "" || e.Type == entryType {
				deletedUUIDs = append(deletedUUIDs, e.UUID)
				delete(db.entries, seq)
				deletedCount++
			}
		}
	}

	for _, u := range deletedUUIDs {
		delete(db.tags, u)
	}

	return deletedCount, nil
}

// CountEntries returns total entries in the database.
func (db *MockTelescopeDatabase) CountEntries() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.entries)
}

// CountTags returns total tags in the database.
func (db *MockTelescopeDatabase) CountTags() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var total int
	for _, tags := range db.tags {
		total += len(tags)
	}
	return total
}

// WatermarkRecord stores watermark metadata in PostgreSQL.
type WatermarkRecord struct {
	ProjectID    int
	EntryType    string
	LastSequence int64
	LastSyncedAt time.Time
}

// MockMetadataDatabase provides an in-memory simulation of PostgreSQL store
// for projects and telescope_watermarks tables.
type MockMetadataDatabase struct {
	mu         sync.RWMutex
	projects   map[string]*store.Project // key: iid
	projectIDs map[int]string            // id -> iid
	watermarks map[string]*WatermarkRecord // key: fmt.Sprintf("%d:%s", projectID, entryType)
	failUpsert bool
}

// NewMockMetadataDatabase instantiates an empty metadata database.
func NewMockMetadataDatabase() *MockMetadataDatabase {
	return &MockMetadataDatabase{
		projects:   make(map[string]*store.Project),
		projectIDs: make(map[int]string),
		watermarks: make(map[string]*WatermarkRecord),
	}
}

// SetFailUpsert configures the mock to fail watermark upserts.
func (m *MockMetadataDatabase) SetFailUpsert(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failUpsert = fail
}

// AddProject registers a project in the mock store.
func (m *MockMetadataDatabase) AddProject(id int, p *store.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.projects[p.IID] = p
	m.projectIDs[id] = p.IID
	return nil
}

// GetProject retrieves a project by its public UUID iid.
func (m *MockMetadataDatabase) GetProject(iid string) (*store.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.projects[iid]
	if !exists {
		return nil, nil
	}
	return p, nil
}

// GetWatermark retrieves the last sequence number for a project and entry type.
func (m *MockMetadataDatabase) GetWatermark(projectID int, entryType string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%d:%s", projectID, entryType)
	if wm, exists := m.watermarks[key]; exists {
		return wm.LastSequence, nil
	}
	return 0, nil
}

// UpsertWatermark updates the watermark monotonically:
// `last_sequence = GREATEST(last_sequence, new_seq)`.
func (m *MockMetadataDatabase) UpsertWatermark(projectID int, entryType string, sequence int64, syncedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failUpsert {
		return errors.New("simulated PostgreSQL metadata transaction failure")
	}

	key := fmt.Sprintf("%d:%s", projectID, entryType)
	wm, exists := m.watermarks[key]
	if !exists {
		m.watermarks[key] = &WatermarkRecord{
			ProjectID:    projectID,
			EntryType:    entryType,
			LastSequence: sequence,
			LastSyncedAt: syncedAt.UTC(),
		}
		return nil
	}

	if sequence > wm.LastSequence {
		wm.LastSequence = sequence
		wm.LastSyncedAt = syncedAt.UTC()
	}

	return nil
}

// GetAllWatermarks returns all watermarks for a project.
func (m *MockMetadataDatabase) GetAllWatermarks(projectID int) map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string]int64)
	prefix := fmt.Sprintf("%d:", projectID)
	for k, wm := range m.watermarks {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			res[wm.EntryType] = wm.LastSequence
		}
	}
	return res
}

// CreateEncryptedProjectHelper creates a store.Project with AES-GCM encrypted credentials.
func CreateEncryptedProjectHelper(iid, name, dsn, storageType, bucket, region, endpoint, accessKey, secretKey string, prune bool) (*store.Project, error) {
	encDSN, err := crypto.Encrypt(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt DSN: %w", err)
	}
	encAccess, err := crypto.Encrypt(accessKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt access key: %w", err)
	}
	encSecret, err := crypto.Encrypt(secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret key: %w", err)
	}

	decDSN, _ := crypto.Decrypt(encDSN)
	decAccess, _ := crypto.Decrypt(encAccess)
	decSecret, _ := crypto.Decrypt(encSecret)

	return &store.Project{
		IID:              iid,
		Name:             name,
		StorageType:      storageType,
		StorageBucket:    bucket,
		StorageRegion:    region,
		StorageEndpoint:  endpoint,
		PruneEnabled:     prune,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		SourceDSN:        decDSN,
		StorageAccessKey: decAccess,
		StorageSecretKey: decSecret,
	}, nil
}
