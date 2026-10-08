package repository

import (
	"fmt"
	"strings"
	"time"
)

func quoteCol(col string, i int) string {
	return fmt.Sprintf("%s = $%d", col, i)
}

func joinSet(set []string) string {
	return strings.Join(set, ", ")
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
