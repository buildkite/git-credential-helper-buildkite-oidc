package repoid

import "strings"

func NormalizeForCache(path string) string {
	normalized := strings.Trim(strings.TrimSpace(path), "/")
	if normalized == "" {
		return ""
	}
	if strings.HasSuffix(normalized, "/info/lfs") {
		normalized = strings.TrimSuffix(normalized, "/info/lfs")
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}
