package utils

import "unicode"

// NormalizeRustDeskID removes presentation whitespace from a RustDesk device
// ID without restricting custom IDs to digits. RustDesk commonly groups a
// numeric ID with spaces when it is copied from the client UI.
func NormalizeRustDeskID(value string) string {
	return string(runeMap(value, func(character rune) bool {
		return unicode.IsSpace(character) || character == '\u200b' || character == '\ufeff'
	}))
}

func runeMap(value string, drop func(rune) bool) []rune {
	result := make([]rune, 0, len(value))
	for _, character := range value {
		if !drop(character) {
			result = append(result, character)
		}
	}
	return result
}
