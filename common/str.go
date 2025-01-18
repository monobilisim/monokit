package common

import (
    "strings"
)

func IsEmptyOrWhitespaceStr(stringValue string) bool {
    return len(strings.TrimSpace(stringValue)) == 0
}
