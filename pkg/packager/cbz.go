package packager

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"comic-crawler/pkg/models"
)

// ComicInfoXML định dạng metadata chuẩn ComicRack/Mihon/Tachiyomi
type ComicInfoXML struct {
	XMLName xml.Name `xml:"ComicInfo"`
	Series  string   `xml:"Series"`
	Title   string   `xml:"Title,omitempty"`
	Number  string   `xml:"Number"`
}

func GenerateComicInfoXML(mangaTitle string, chapterNum float64, chapterTitle string) ([]byte, error) {
	info := ComicInfoXML{
		Series: mangaTitle,
		Title:  chapterTitle,
		Number: fmt.Sprintf("%g", chapterNum),
	}
	data, err := xml.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}
	header := []byte(xml.Header)
	return append(header, data...), nil
}

// CreateCBZFromMemory tạo file .cbz trực tiếp từ các slice bytes trong RAM
// Không ghi bất kỳ file ảnh tạm nào xuống ổ cứng
func CreateCBZFromMemory(
	cbzPath string,
	pages []models.PageData,
	mangaTitle string,
	chapterNum float64,
	chapterTitle string,
) error {
	dir := filepath.Dir(cbzPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("không thể tạo thư mục đích: %w", err)
	}

	outFile, err := os.Create(cbzPath)
	if err != nil {
		return fmt.Errorf("không thể tạo file CBZ: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// 1. Thêm ComicInfo.xml metadata
	if mangaTitle != "" {
		xmlBytes, err := GenerateComicInfoXML(mangaTitle, chapterNum, chapterTitle)
		if err == nil {
			header := &zip.FileHeader{
				Name:   "ComicInfo.xml",
				Method: zip.Store, // Không nén để máy đọc sách đọc metadata tức thì
			}
			w, err := zipWriter.CreateHeader(header)
			if err == nil {
				_, _ = w.Write(xmlBytes)
			}
		}
	}

	// 2. Ghi từng trang ảnh trực tiếp từ bộ nhớ RAM
	for _, page := range pages {
		if len(page.Data) == 0 {
			continue
		}
		header := &zip.FileHeader{
			Name:   page.Filename,
			Method: zip.Store, // Đọc tức thì, 0ms latency, tiết kiệm CPU và pin máy đọc sách
		}
		w, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("lỗi ghi header trang %s: %w", page.Filename, err)
		}
		if _, err := w.Write(page.Data); err != nil {
			return fmt.Errorf("lỗi ghi dữ liệu trang %s: %w", page.Filename, err)
		}
	}

	return nil
}

