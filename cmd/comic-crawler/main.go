package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"comic-crawler/pkg/downloader"
	"comic-crawler/pkg/models"
	"comic-crawler/pkg/parsers"
	"comic-crawler/pkg/utils"
)

func main() {
	// Bắt tín hiệu Ctrl+C để thoát an toàn
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n👋 Đã dừng chương trình. Tạm biệt!")
		os.Exit(0)
	}()

	printBanner()

	scanner := bufio.NewScanner(os.Stdin)

	// VÒNG LẶP CHÍNH (CONTINUOUS LOOP): Cào xong 1 bộ -> tự động hỏi bộ tiếp theo
	for {
		fmt.Println("================================================================================")
		fmt.Print("👉 Nhập URL truyện (hoặc gõ 'exit'/'quit' để thoát): ")
		if !scanner.Scan() {
			break
		}
		rawURL := strings.TrimSpace(scanner.Text())
		if rawURL == "" {
			continue
		}
		if strings.EqualFold(rawURL, "exit") || strings.EqualFold(rawURL, "quit") || strings.EqualFold(rawURL, "q") {
			fmt.Println("\n👋 Đã thoát chương trình. Hẹn gặp lại!")
			break
		}

		// Hỏi thư mục lưu truyện
		fmt.Printf("📂 Thư mục lưu truyện [Nhấn Enter để chọn mặc định: %s]: ", utils.DefaultComicDir)
		scanner.Scan()
		rawOutDir := strings.TrimSpace(scanner.Text())
		outputDir := utils.ResolveOutputDir(rawOutDir)
		fmt.Printf("📁 Thư mục lưu đích: %s\n\n", outputDir)

		// Quét thông tin truyện
		fmt.Printf("🔍 Đang kết nối và quét thông tin truyện từ: %s ...\n", rawURL)
		manga, err := parsers.DefaultExtractor.ExtractManga(rawURL)
		if err != nil {
			fmt.Printf("❌ Lỗi khi lấy thông tin truyện: %v\n\n", err)
			continue
		}

		fmt.Printf("📚 Tên truyện: %s\n", manga.Title)
		fmt.Printf("📑 Tổng số chương tìm thấy: %d\n\n", len(manga.Chapters))

		if len(manga.Chapters) == 0 {
			fmt.Println("❌ Không tìm thấy chương nào từ URL này. Vui lòng kiểm tra lại link.\n")
			continue
		}

		// In bảng vài chapter mẫu
		printChapterSample(manga.Chapters)

		// Chọn chương để tải
		fmt.Print("\n👉 Chọn chương cần tải ('all' để tải hết, '1-10', '50,52', hoặc số chương) [Mặc định: all]: ")
		scanner.Scan()
		chapChoice := strings.TrimSpace(scanner.Text())
		if chapChoice == "" {
			chapChoice = "all"
		}

		selectedChapters := parseChapterSelection(chapChoice, manga.Chapters)
		fmt.Printf("🚀 Số chương đã chọn để tải: %d\n\n", len(selectedChapters))

		// Khởi tạo Go Concurrency Downloader (8 workers tải ảnh song song siêu tốc)
		dl := downloader.NewDownloader(outputDir, 8)
		safeMangaTitle := utils.CleanFilename(manga.Title)
		finalMangaFolder := filepath.Join(outputDir, safeMangaTitle)

		fmt.Printf("⬇ Bắt đầu tải và nén trực tiếp thành .cbz vào: %s\n", finalMangaFolder)
		totalChaps := len(selectedChapters)

		for idx, ch := range selectedChapters {
			fmt.Printf("\n[%d/%d] 📦 Đang xử lý: %s (Chap %g)\n", idx, totalChaps, ch.Title, ch.Number)

			// Nếu chapter chưa có link ảnh, bóc tách ảnh
			if len(ch.Pages) == 0 {
				fmt.Print("  🔎 Đang lấy danh sách trang ảnh...")
				pages, err := parsers.DefaultExtractor.ExtractChapterImages(ch.URL)
				if err != nil || len(pages) == 0 {
					fmt.Printf(" ⚠ Không lấy được ảnh của chương này: %v\n", err)
					continue
				}
				ch.Pages = pages
				fmt.Printf(" (Tìm thấy %d trang)\n", len(pages))
			} else {
				fmt.Printf("  ✓ Có %d trang ảnh\n", len(ch.Pages))
			}

			// Tải và nén CBZ siêu tốc bằng Goroutines trong RAM
			cbzDest, err := dl.DownloadChapter(manga, &ch)
			if err != nil {
				fmt.Printf("  ❌ Lỗi tải chương: %v\n", err)
			} else {
				fmt.Printf("  🎉 Hoàn tất file: %s\n", cbzDest)
			}
		}

		// In thông báo hoàn thành bộ truyện này
		printSuccessSummary(manga.Title, finalMangaFolder)
	}
}

func printBanner() {
	fmt.Println("================================================================================")
	fmt.Println("      🚀 COMIC CRAWLER ENGINE (GOLANG ULTRA-FAST & RECURRENT LOOP) 🚀")
	fmt.Println("  Tải ảnh Goroutines -> Nén CBZ trong RAM -> Chuẩn máy đọc sách Android")
	fmt.Println("  Chạy vòng lặp liên tục, cào xong hỏi tiếp URL truyện mới (gõ 'exit' để thoát)")
	fmt.Println("================================================================================")
	fmt.Println()
}

func printChapterSample(chapters []models.Chapter) {
	fmt.Println("--- DANH SÁCH CHƯƠNG (MẪU) ---")
	limit := 5
	if len(chapters) < limit {
		limit = len(chapters)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("  [%03d] %s (Chap %g)\n", i+1, chapters[i].Title, chapters[i].Number)
	}
	if len(chapters) > 10 {
		fmt.Println("  ...")
		for i := len(chapters) - 2; i < len(chapters); i++ {
			fmt.Printf("  [%03d] %s (Chap %g)\n", i+1, chapters[i].Title, chapters[i].Number)
		}
	}
}

func parseChapterSelection(input string, allChapters []models.Chapter) []models.Chapter {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" || input == "all" {
		return allChapters
	}

	selectedMap := make(map[string]models.Chapter)
	tokens := strings.Split(input, ",")

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if strings.Contains(token, "-") {
			parts := strings.Split(token, "-")
			if len(parts) == 2 {
				start, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				end, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err1 == nil && err2 == nil {
					for _, ch := range allChapters {
						if ch.Number >= start && ch.Number <= end {
							selectedMap[ch.URL] = ch
						}
					}
				}
			}
		} else {
			if num, err := strconv.ParseFloat(token, 64); err == nil {
				found := false
				for _, ch := range allChapters {
					if ch.Number == num {
						selectedMap[ch.URL] = ch
						found = true
					}
				}
				if !found {
					idx := int(num) - 1
					if idx >= 0 && idx < len(allChapters) {
						selectedMap[allChapters[idx].URL] = allChapters[idx]
					}
				}
			}
		}
	}

	if len(selectedMap) == 0 {
		return allChapters
	}

	var res []models.Chapter
	for _, ch := range allChapters {
		if _, ok := selectedMap[ch.URL]; ok {
			res = append(res, ch)
		}
	}
	return res
}

func printSuccessSummary(title, finalFolder string) {
	fmt.Println("\n--------------------------------------------------------------------------------")
	fmt.Printf("🎉 ĐÃ HOÀN TẤT BỘ TRUYỆN: %s\n", title)
	fmt.Printf("📂 Thư mục chứa các file .cbz: %s\n", finalFolder)
	fmt.Println("\n📱 HƯỚNG DẪN CHÉP VÀO MÁY ĐỌC SÁCH ANDROID:")
	fmt.Println("  1. Cắm cáp USB máy đọc sách vào máy tính (chế độ Truyền tệp / MTP).")
	fmt.Printf("  2. Chép thư mục \"%s\" vào:\n", filepath.Base(finalFolder))
	fmt.Printf("     • Mihon / Tachiyomi: Bộ nhớ máy -> Mihon/local/%s\n", filepath.Base(finalFolder))
	fmt.Println("       (Vào app -> Duyệt (Browse) -> Nguồn cục bộ (Local source) là đọc được ngay).")
	fmt.Println("     • Moon+ Reader / Perfect Viewer: Chép vào Books/ hoặc Comics/ rồi mở app đọc.")
	fmt.Println("--------------------------------------------------------------------------------\n")
}

