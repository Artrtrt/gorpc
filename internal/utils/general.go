package utils

import (
	"bytes"
	"strings"
)

func ByteArrToString(arr []byte) string {
	return string(bytes.TrimRightFunc(arr, func(r rune) bool {
		return r == 0
	}))
}

func MagicSNTransform(SN string) string {
	runes := []rune(SN)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func Contains(arr []string, str string) bool {
	for _, val := range arr {
		if strings.Contains(val, str) {
			return true
		}
	}
	return false
}
