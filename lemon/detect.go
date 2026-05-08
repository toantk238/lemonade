package lemon

// DetectFileExt returns the file extension if b begins with known image magic
// bytes. Returns ("", false) for non-image data or insufficient bytes.
func DetectFileExt(b []byte) (ext string, ok bool) {
	switch {
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return ".png", true
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return ".jpg", true
	case len(b) >= 4 && b[0] == 'G' && b[1] == 'I' && b[2] == 'F' && b[3] == '8':
		return ".gif", true
	case len(b) >= 2 && b[0] == 'B' && b[1] == 'M':
		return ".bmp", true
	case len(b) >= 12 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P':
		return ".webp", true
	default:
		return "", false
	}
}
