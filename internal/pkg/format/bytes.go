package format

import "fmt"

// Bytes converts a byte count to a human-readable string (e.g. "1.50MB").
func Bytes(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(bytes)
	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.2f%s", size, units[unitIndex])
}
