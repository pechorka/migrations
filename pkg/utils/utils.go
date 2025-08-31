package utils

import (
	"strings"
)

func QuoteIdentBacktick(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
