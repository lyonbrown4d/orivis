package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func dashboardSnapshotETag(snapshot dashboardSnapshotResponse) (string, error) {
	snapshot.GeneratedAt = time.Time{}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal dashboard snapshot etag payload: %w", err)
	}
	sum := sha256.Sum256(raw)
	return `W/"` + hex.EncodeToString(sum[:16]) + `"`, nil
}

func dashboardETagMatches(header, etag string) bool {
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
