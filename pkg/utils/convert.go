package utils

import (
	"unsafe"
)

// BytesToReadOnlyString returns a string converted from given bytes.
func BytesToReadOnlyString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// StringToReadOnlyBytes returns bytes converted from given string.
func StringToReadOnlyBytes(s string) (bs []byte) {
	if len(s) == 0 {
		return nil
	}
	bs = unsafe.Slice(unsafe.StringData(s), len(s))
	return
}
