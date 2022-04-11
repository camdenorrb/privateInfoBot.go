package utils

import "strings"

func SubstringAfter(original string, substr string) string {

	index := strings.Index(original, substr)
	if index == -1 {
		return original
	}

	return original[index+len(substr):]
}

func SubstringAfterLast(original string, substr string) string {

	index := strings.LastIndex(original, substr)
	if index == -1 {
		return original
	}

	return original[index+len(substr):]
}

func SubstringBefore(original string, substr string) string {

	index := strings.Index(original, substr)
	if index == -1 {
		return original
	}

	return original[:index]
}

func SubstringBeforeLast(original string, substr string) string {

	index := strings.LastIndex(original, substr)
	if index == -1 {
		return original
	}

	return original[:index]
}
