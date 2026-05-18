package cache

import "fmt"

func DashboardSnapshotKey(resultLimit int) string {
	return fmt.Sprintf("dashboard:snapshot:%d", resultLimit)
}
