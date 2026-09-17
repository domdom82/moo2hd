package main

import (
	"fmt"
	"strings"
)

const hexDumpMaxBytes = 512

// hexDump returns an xxd-style hex+ASCII dump of data, capped at hexDumpMaxBytes.
func hexDump(data []byte, bytesPerLine int) string {
	if bytesPerLine < 1 {
		bytesPerLine = 16
	}
	if len(data) > hexDumpMaxBytes {
		data = data[:hexDumpMaxBytes]
	}

	var sb strings.Builder
	for i := 0; i < len(data); i += bytesPerLine {
		end := i + bytesPerLine
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]

		fmt.Fprintf(&sb, "%08x  ", i)
		for j, b := range chunk {
			fmt.Fprintf(&sb, "%02x ", b)
			if j == bytesPerLine/2-1 {
				sb.WriteByte(' ')
			}
		}
		// Pad short last line.
		pad := bytesPerLine - len(chunk)
		for range pad {
			sb.WriteString("   ")
		}
		if pad > 0 && bytesPerLine/2 > len(chunk) {
			sb.WriteByte(' ')
		}
		sb.WriteString(" |")
		for _, b := range chunk {
			if b >= 0x20 && b < 0x7f {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	if len(data) == hexDumpMaxBytes {
		fmt.Fprintf(&sb, "... (truncated at %d bytes)\n", hexDumpMaxBytes)
	}
	return sb.String()
}
