package downloader

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"comic-crawler/pkg/models"
	"comic-crawler/pkg/network"
	"comic-crawler/pkg/packager"
	"comic-crawler/pkg/utils"
)

type Downloader struct {
	outputDir   string
	concurrency int
	fetcher     *network.Fetcher
}

func NewDownloader(outputDir string, concurrency int) *Downloader {
	if concurrency <= 0 {
		concurrency = 8
	}
	return &Downloader{
		outputDir:   utils.ResolveOutputDir(outputDir),
		concurrency: concurrency,
		fetcher:     network.DefaultFetcher,
	}
}

// DownloadChapter tải toàn bộ các trang ảnh của chương vào RAM bằng Goroutine Pool, nén trực tiếp thành .cbz
func (d *Downloader) DownloadChapter(manga *models.Manga, chapter *models.Chapter) (string, error) {
	safeMangaTitle := utils.CleanFilename(manga.Title)
	safeChapTitle := utils.FormatChapterName(chapter.Number, chapter.Title)

	mangaDir := filepath.Join(d.outputDir, safeMangaTitle)
	cbzPath := filepath.Join(mangaDir, fmt.Sprintf("%s.cbz", safeChapTitle))

	pages := chapter.Pages
	total := len(pages)
	if total == 0 {
		return "", fmt.Errorf("chương '%s' không có trang ảnh nào", chapter.Title)
	}

	// Xác định Referer chuẩn từ domain hoạt động của bộ truyện
	referer := manga.URL
	if u, err := url.Parse(manga.URL); err == nil && u.Scheme != "" && u.Host != "" {
		referer = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	}

	type job struct {
		index int
		url   string
	}

	jobs := make(chan job, total)
	results := make(chan models.PageData, total)

	var wg sync.WaitGroup
	var completedCount int64

	// Khởi chạy Goroutine Worker Pool
	for w := 0; w < d.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var data []byte
				var err error

				// Retry tối đa 3 lần cho mỗi ảnh
				for attempt := 0; attempt < 3; attempt++ {
					data, err = d.fetcher.FetchBytes(j.url, referer)
					if err == nil && len(data) > 0 {
						break
					}
					time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				}

				if err == nil && len(data) > 0 {
					ext := utils.DetectImageExtension(data)
					filename := utils.FormatPageFilename(j.index, ext)
					results <- models.PageData{
						Index:    j.index,
						Filename: filename,
						Data:     data,
					}
				}

				atomic.AddInt64(&completedCount, 1)
				current := atomic.LoadInt64(&completedCount)
				printProgressBar(int(current), total)
			}
		}()
	}

	// Đẩy các trang vào hàng đợi
	for i, pageURL := range pages {
		jobs <- job{index: i + 1, url: pageURL}
	}
	close(jobs)

	// Đợi tất cả workers hoàn thành
	wg.Wait()
	close(results)
	fmt.Println() // Xuống dòng sau progress bar

	// Thu thập các trang đã tải
	var downloadedPages []models.PageData
	for p := range results {
		downloadedPages = append(downloadedPages, p)
	}

	if len(downloadedPages) == 0 {
		return "", fmt.Errorf("không thể tải bất kỳ trang ảnh nào của chương '%s'", chapter.Title)
	}

	// Sắp xếp các trang đúng thứ tự 1, 2, 3...
	sort.Slice(downloadedPages, func(i, j int) bool {
		return downloadedPages[i].Index < downloadedPages[j].Index
	})

	// Đóng gói trực tiếp thành file .cbz
	err := packager.CreateCBZFromMemory(
		cbzPath,
		downloadedPages,
		manga.Title,
		chapter.Number,
		chapter.Title,
	)
	if err != nil {
		return "", fmt.Errorf("lỗi đóng gói file CBZ: %w", err)
	}

	return cbzPath, nil
}

func printProgressBar(current, total int) {
	barWidth := 30
	percent := float64(current) / float64(total)
	filled := int(percent * float64(barWidth))

	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else if i == filled {
			bar += "▌"
		} else {
			bar += "░"
		}
	}

	fmt.Printf("\r  ⏳ [%s] %d/%d trang (%.0f%%)", bar, current, total, percent*100)
	_ = os.Stdout.Sync()
}
