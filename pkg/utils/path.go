package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var illegalPathChars = regexp.MustCompile(`[\\/*?:"<>|]`)
var whitespaceRegex = regexp.MustCompile(`[\r\n\t\s]+`)
var chapterPrefixRegex = regexp.MustCompile(`(?i)^(Chương|Chapter|Chap)\s*\d+([\s:\-_]+)?`)

// GetDefaultComicDir tự động xác định thư mục 'comic' cùng cấp với thư mục tool (không nằm trong source)
func GetDefaultComicDir() string {
	var toolDir string
	if exePath, err := os.Executable(); err == nil {
		toolDir = filepath.Dir(exePath)
	} else {
		toolDir, _ = os.Getwd()
	}

	parentDir := filepath.Dir(toolDir)
	// Tránh trường hợp tool nằm ở thư mục gốc ổ đĩa (vd: C:\ hoặc D:\)
	if parentDir == toolDir || parentDir == "" || parentDir == "." {
		return filepath.Join(toolDir, "comic")
	}

	return filepath.Join(parentDir, "comic")
}

// ResolveOutputDir xác định đường dẫn thư mục lưu truyện
// Nếu rỗng -> Lấy thư mục 'comic' mặc định cùng cấp với tool
// Nếu tương đối (vd: homunculus) -> [DefaultComicDir]\homunculus
// Nếu tuyệt đối (vd: D:\Truyen) -> giữ nguyên
func ResolveOutputDir(userInput string) string {
	defaultDir := GetDefaultComicDir()
	cleaned := strings.TrimSpace(userInput)
	cleaned = strings.Trim(cleaned, "\ufeff\u200b\u200c\u200d\u00a0\"'")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		_ = os.MkdirAll(defaultDir, 0755)
		return defaultDir
	}

	if filepath.IsAbs(cleaned) {
		res := filepath.Clean(cleaned)
		_ = os.MkdirAll(res, 0755)
		return res
	}

	res := filepath.Clean(filepath.Join(defaultDir, cleaned))
	_ = os.MkdirAll(res, 0755)
	return res
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

