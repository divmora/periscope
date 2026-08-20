package collector

import (
	"fmt"

	"github.com/google/uuid"
)

// PartitionKey uniquely identifies a Hive partition bucket by project, type, and hourly time slice in UTC.
type PartitionKey struct {
	ProjectIID string
	Type       string
	Year       int
	Month      int
	Day        int
	Hour       int
}

// PartitionKeyFromRecord derives a PartitionKey from a TelemetryRecord and project IID, strictly normalizing to UTC.
func PartitionKeyFromRecord(projectIID string, rec TelemetryRecord) PartitionKey {
	utc := rec.CreatedAt.UTC()
	return PartitionKey{
		ProjectIID: projectIID,
		Type:       rec.Type,
		Year:       utc.Year(),
		Month:      int(utc.Month()),
		Day:        utc.Day(),
		Hour:       utc.Hour(),
	}
}

// PartitionDirectory returns the Hive partition prefix directory path.
// Format: projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/
func (k PartitionKey) PartitionDirectory() string {
	return fmt.Sprintf("projectId=%s/type=%s/year=%04d/month=%02d/day=%02d/hour=%02d/",
		k.ProjectIID, k.Type, k.Year, k.Month, k.Day, k.Hour)
}

// PartitionFileName returns the standardized Parquet filename for a partition batch.
// Format: part-{minSeq:012d}-{maxSeq:012d}-{batchUUID}.parquet
func PartitionFileName(minSeq, maxSeq int64, batchUUID string) string {
	if batchUUID == "" {
		batchUUID = uuid.NewString()
	}
	return fmt.Sprintf("part-%012d-%012d-%s.parquet", minSeq, maxSeq, batchUUID)
}

// PartitionPath returns the full Hive-partitioned object storage key for a batch.
// Format: projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/part-{minSeq:012d}-{maxSeq:012d}-{uuid}.parquet
func PartitionPath(key PartitionKey, minSeq, maxSeq int64, batchUUID string) string {
	return key.PartitionDirectory() + PartitionFileName(minSeq, maxSeq, batchUUID)
}
