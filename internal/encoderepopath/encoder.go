package encoderepopath

import (
	"fmt"
	"strings"
)

func isSafe(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '.'
}

// Encode converts a repository URL into a reversible local directory name.
func Encode(urlStr string) string {
	var builder strings.Builder

	for idx := 0; idx < len(urlStr); idx += 1 {
		ch := urlStr[idx]

		switch {
		case isSafe(ch):
			builder.WriteByte(ch)
		case ch == '/':
			builder.WriteByte('_')
		default:
			// "-" + 2-digit hex (percent-encoding style, but % -> -)
			_, err := fmt.Fprintf(&builder, "-%02X", ch)
			if err != nil {
				return ""
			}
		}
	}

	return builder.String()
}

// Decode restores the original string from Encode().
func Decode(encoded string) (string, error) {
	var builder strings.Builder

	for idx := 0; idx < len(encoded); idx += 1 {
		ch := encoded[idx]

		switch {
		case ch == '_':
			builder.WriteByte('/')
		case ch == '-':
			if idx+2 >= len(encoded) {
				return "", fmt.Errorf("invalid escape at position %d", idx)
			}

			hexPart := encoded[idx+1 : idx+3]
			var chFromHex byte
			if _, err := fmt.Sscanf(hexPart, "%02X", &chFromHex); err != nil {
				return "", fmt.Errorf("invalid hex escape -%s at position %d", hexPart, idx)
			}

			builder.WriteByte(chFromHex)
			idx += 2
		default:
			builder.WriteByte(ch)
		}
	}

	return builder.String(), nil
}
