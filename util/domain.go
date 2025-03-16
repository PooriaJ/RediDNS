package util

import (
	"strings"
)

// FormatRecordName formats a record name based on the zone name
// If the record name is '@', it returns the zone name
// If the record name already ends with the zone name, it returns it as is
// Otherwise, it appends the zone name to the record name
func FormatRecordName(recordName, zoneName string) string {
	// Handle '@' symbol for root domain
	if recordName == "@" {
		return zoneName
	}

	// If the record name already ends with the zone name, return it as is
	if strings.HasSuffix(recordName, zoneName) {
		return recordName
	}

	// If the record name already contains dots, assume it's a fully qualified domain name
	if strings.Contains(recordName, ".") {
		return recordName
	}

	// Otherwise, append the zone name to the record name
	return recordName + "." + zoneName
}
