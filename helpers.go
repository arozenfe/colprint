package colprint

// padLeft appends s to dst, padding or truncating to width characters.
// All padding is on the right (left-aligned text).
func padLeft(dst []byte, s string, width int) []byte {
	return padBytesLeft(dst, []byte(s), width)
}

// padBytesLeft appends val to dst, padding or truncating to width.
// This operates on byte slices for efficiency.
func padBytesLeft(dst, val []byte, width int) []byte {
	// Truncate if too long
	if len(val) > width {
		return append(dst, val[:width]...)
	}

	// Append value
	dst = append(dst, val...)

	// Pad with spaces on the right
	for i := len(val); i < width; i++ {
		dst = append(dst, ' ')
	}

	return dst
}

// Phase 2: Right-alignment functions (to be implemented)
//
// func padRight(dst []byte, s string, width int) []byte {
//     return padBytesRight(dst, []byte(s), width)
// }
//
// func padBytesRight(dst, val []byte, width int) []byte {
//     // Truncate if too long
//     if len(val) > width {
//         return append(dst, val[:width]...)
//     }
//
//     // Pad with spaces on the left
//     if pad := width - len(val); pad > 0 {
//         for i := 0; i < pad; i++ {
//             dst = append(dst, ' ')
//         }
//     }
//
//     // Append value
//     return append(dst, val...)
// }
