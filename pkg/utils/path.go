package utils

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const DefaultComicDir = `C:\Users\ROSA\workspace\comic`

var illegalPathChars = regexp.MustCompile(`[\\/*?:"<>|]`)
var whitespaceRegex = regexp.MustCompile(`[\r\n\t\s]+`)
var chapterPrefixRegex = regexp.MustCompile(`(?i)^(Chương|Chapter|Chap)\s*\d+([\s:\-_]+)?`)

// ResolveOutputDir xác định đường dẫn thư mục lưu truyện
// Nếu rỗng -> C:\Users\ROSA\workspace\comic
// Nếu tương đối (vd: 20centuryboys) -> C:\Users\ROSA\workspace\comic\20centuryboys
// Nếu tuyệt đối (vd: D:\Truyen) -> giữ nguyên
func ResolveOutputDir(userInput string) string {
	cleaned := strings.TrimSpace(userInput)
	cleaned = strings.Trim(cleaned, `"'`)
	if cleaned == "" {
		return DefaultComicDir
	}

	if filepath.IsAbs(cleaned) {
		return filepath.Clean(cleaned)
	}

	return filepath.Clean(filepath.Join(DefaultComicDir, cleaned))
}

// CleanFilename làm sạch tên file/folder tương thích hệ điều hành Windows & Android
func CleanFilename(name string, maxLen ...int) string {
	limit := 150
	if len(maxLen) > 0 && maxLen[0] > 0 {
		limit = maxLen[0]
	}

	cleaned := illegalPathChars.ReplaceAllString(name, "_")
	cleaned = whitespaceRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.Trim(cleaned, ". ")
	if cleaned == "" {
		cleaned = "unnamed"
	}

	runes := []rune(cleaned)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return cleaned
}

// FormatChapterName format tên chapter có zero-padding 3 chữ số: 'Chapter 001 - Tên'
func FormatChapterName(number float64, title string) string {
	var numStr string
	if number == float64(int64(number)) {
		numStr = fmt.Sprintf("%03d", int64(number))
	} else {
		numStr = fmt.Sprintf("%06.2f", number)
		numStr = strings.TrimRight(strings.TrimRight(numStr, "0"), ".")
	}

	base := fmt.Sprintf("Chapter %s", numStr)
	if title != "" {
		cleanTitle := chapterPrefixRegex.ReplaceAllString(title, "")
		cleanTitle = strings.TrimSpace(cleanTitle)
		if cleanTitle != "" {
			base = fmt.Sprintf("%s - %s", base, cleanTitle)
		}
	}

	return CleanFilename(base)
}

// FormatPageFilename đánh số trang 3 chữ số: 001.jpg, 002.jpg
func FormatPageFilename(index int, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%03d%s", index, strings.ToLower(ext))
}

// DetectImageExtension phát hiện đuôi ảnh dựa trên magic bytes
func DetectImageExtension(data []byte) string {
	if len(data) >= 3 && bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		return ".jpg"
	}
	if len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return ".png"
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[:12], []byte("WEBP")) {
		return ".webp"
	}
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("GIF8")) {
		return ".gif"
	}
	return ".jpg"
}

