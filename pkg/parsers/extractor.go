package parsers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"comic-crawler/pkg/models"
	"comic-crawler/pkg/network"

	"github.com/PuerkitoBio/goquery"
)

var (
	chapNumRegex = regexp.MustCompile(`(?i)(?:chương|chapter|chap|tập|ep|c)\s*[:.\-_]?\s*(\d+(?:\.\d+)?)`)
	urlNumRegex  = regexp.MustCompile(`(?i)[-/](?:chap|chapter|chuong|c)-?(\d+(?:\.\d+)?)`)
	cleanTitleRe = regexp.MustCompile(`(?i)[\-|–|]\s*(NetTruyen|TruyenQQ|BlogTruyen|MangaDex|Webtoon|Truyện Tranh Online|Đọc Truyện).*$`)
	badImgRegex  = regexp.MustCompile(`(?i)(logo|banner|facebook|discord|donate|icon|advert|loading|placeholder|button|avatar|tracking|pixel)`)
)

type Extractor struct {
	fetcher *network.Fetcher
}

var DefaultExtractor = &Extractor{fetcher: network.DefaultFetcher}

func ExtractNumber(text string) (float64, bool) {
	if m := chapNumRegex.FindStringSubmatch(text); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return n, true
		}
	}
	if m := urlNumRegex.FindStringSubmatch(text); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// ExtractManga phân tích và bóc tách thông tin bộ truyện từ URL
func (e *Extractor) ExtractManga(targetURL string) (*models.Manga, error) {
	finalURL, html, err := e.fetcher.FetchHTML(targetURL)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("lỗi phân tích cú pháp HTML: %w", err)
	}

	u, _ := url.Parse(finalURL)
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// Chuẩn hóa sang domain hoạt động thông qua og:url hoặc link canonical nếu có
	if ogURL, ok := doc.Find(`meta[property="og:url"]`).Attr("content"); ok && strings.HasPrefix(ogURL, "http") {
		if oU, err := url.Parse(ogURL); err == nil && oU.Host != "" {
			finalURL = ogURL
			u = oU
			baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	} else if canURL, ok := doc.Find(`link[rel="canonical"]`).Attr("href"); ok && strings.HasPrefix(canURL, "http") {
		if cU, err := url.Parse(canURL); err == nil && cU.Host != "" {
			finalURL = canURL
			u = cU
			baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}

	// 1. Trích xuất Tiêu đề truyện
	title := ""
	if ogTitle, ok := doc.Find(`meta[property="og:title"]`).Attr("content"); ok && strings.TrimSpace(ogTitle) != "" {
		title = strings.TrimSpace(ogTitle)
	}
	if title == "" {
		h1 := doc.Find("h1.title-detail, h1.title, h1").First()
		if h1.Length() > 0 {
			title = strings.TrimSpace(h1.Text())
		}
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").Text())
	}
	title = cleanTitleRe.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)
	if title == "" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		title = "Comic_" + parts[len(parts)-1]
	}

	// 2. Trích xuất Cover Image
	coverURL := ""
	if ogImg, ok := doc.Find(`meta[property="og:image"]`).Attr("content"); ok {
		coverURL = resolveURL(baseURL, ogImg)
	}

	// 3. Trích xuất Chapters (Thử qua Dynamic API trước)
	slug := extractSlug(u.Path)
	chapters := tryDynamicComicService(baseURL, finalURL, slug)

	// 4. Nếu không có API, quét DOM
	if len(chapters) == 0 {
		chapters = scanDOMChapters(doc, baseURL, finalURL)
	}

	// 5. Nếu không tìm thấy chapter nào, kiểm tra xem có phải URL của 1 chương đọc lẻ không
	if len(chapters) == 0 {
		imgs := extractImagesFromDoc(doc, finalURL)
		if len(imgs) > 0 {
			num, _ := ExtractNumber(finalURL)
			if num == 0 {
				num, _ = ExtractNumber(title)
			}
			if num == 0 {
				num = 1.0
			}
			chapters = append(chapters, models.Chapter{
				ID:     "chap_1",
				Title:  title,
				URL:    finalURL,
				Number: num,
				Pages:  imgs,
			})
		}
	}

	// Sắp xếp các chương tăng dần theo số
	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Number < chapters[j].Number
	})

	return &models.Manga{
		Title:    title,
		URL:      finalURL,
		CoverURL: coverURL,
		Chapters: chapters,
	}, nil
}

// ExtractChapterImages bóc tách danh sách link ảnh của 1 chương
func (e *Extractor) ExtractChapterImages(chapterURL string) ([]string, error) {
	finalURL, html, err := e.fetcher.FetchHTML(chapterURL, chapterURL)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	if ogURL, ok := doc.Find(`meta[property="og:url"]`).Attr("content"); ok && strings.HasPrefix(ogURL, "http") {
		finalURL = ogURL
	} else if canURL, ok := doc.Find(`link[rel="canonical"]`).Attr("href"); ok && strings.HasPrefix(canURL, "http") {
		finalURL = canURL
	}

	return extractImagesFromDoc(doc, finalURL), nil
}

func extractSlug(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "truyen-tranh" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func tryDynamicComicService(baseURL, referer, slug string) []models.Chapter {
	if slug == "" {
		return nil
	}
	apiURL := fmt.Sprintf("%s/Comic/Services/ComicService.asmx/ChapterList?slug=%s", baseURL, url.QueryEscape(slug))
	_, html, err := network.DefaultFetcher.FetchHTML(apiURL, referer)
	if err != nil {
		return nil
	}

	type apiItem struct {
		ChapterNum  float64 `json:"chapter_num"`
		ChapterName string  `json:"chapter_name"`
		ChapterSlug string  `json:"chapter_slug"`
	}
	type apiResp struct {
		Data []apiItem `json:"data"`
	}

	var apiResult apiResp
	if err := json.Unmarshal([]byte(html), &apiResult); err == nil && len(apiResult.Data) > 0 {
		var chapters []models.Chapter
		for _, it := range apiResult.Data {
			cSlug := it.ChapterSlug
			if cSlug == "" {
				cSlug = fmt.Sprintf("chuong-%g", it.ChapterNum)
			}
			chURL := fmt.Sprintf("%s/truyen-tranh/%s/%s", baseURL, slug, cSlug)
			name := it.ChapterName
			if name == "" {
				name = fmt.Sprintf("Chapter %g", it.ChapterNum)
			}
			chapters = append(chapters, models.Chapter{
				ID:     chURL,
				Title:  name,
				URL:    chURL,
				Number: it.ChapterNum,
			})
		}
		return chapters
	}
	return nil
}

func scanDOMChapters(doc *goquery.Document, baseURL, currentURL string) []models.Chapter {
	var chapters []models.Chapter
	seen := make(map[string]bool)

	selector := ".works-chapter-list a, .list-chapter a, .chapter-list a, #chapterlist a, " +
		".chapters a, .listing-chapters_wrap a, .wp-manga-chapter a, #list-chapter a, " +
		".box-list-chapter a, ul.list-chapters a, #examples a, .list-wrap a, #nt_listchapter a, #chapter_list a"

	sel := doc.Find(selector)
	if sel.Length() == 0 {
		sel = doc.Find("a[href]")
	}

	sel.Each(func(i int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "#") {
			return
		}

		fullURL := resolveURL(currentURL, href)
		if seen[fullURL] {
			return
		}

		txt := strings.TrimSpace(s.Text())
		hasChapInHref := urlNumRegex.MatchString(href)
		hasChapInText := chapNumRegex.MatchString(txt)

		if hasChapInHref || hasChapInText {
			seen[fullURL] = true
			num, _ := ExtractNumber(txt)
			if num == 0 {
				num, _ = ExtractNumber(href)
			}

			title := txt
			if title == "" {
				title = fmt.Sprintf("Chapter %g", num)
			}

			chapters = append(chapters, models.Chapter{
				ID:     fullURL,
				Title:  title,
				URL:    fullURL,
				Number: num,
			})
		}
	})

	return chapters
}

func extractImagesFromDoc(doc *goquery.Document, baseURL string) []string {
	var urls []string
	seen := make(map[string]bool)

	containers := doc.Find(".story-see-content, .reading-detail, .page-chapter, .chapter-content, .reading-content, #readerarea, .vung-doc, #viewer, .image-container, .entry-content, .box-read-content, #chapter-content, .read-content")
	var imgSel *goquery.Selection
	if containers.Length() > 0 {
		imgSel = containers.Find("img")
	} else {
		imgSel = doc.Find("img")
	}

	imgAttrs := []string{"data-original", "data-src", "data-cdn", "data-lazy-src", "data-url", "data-sv1", "data-sv2", "src"}

	imgSel.Each(func(i int, s *goquery.Selection) {
		var src string
		for _, attr := range imgAttrs {
			if val, ok := s.Attr(attr); ok && strings.TrimSpace(val) != "" {
				src = strings.TrimSpace(val)
				break
			}
		}

		if src == "" {
			return
		}

		if strings.HasPrefix(src, "//") {
			src = "https:" + src
		}

		fullURL := resolveURL(baseURL, src)
		lower := strings.ToLower(fullURL)

		if badImgRegex.MatchString(lower) {
			return
		}

		if !seen[fullURL] {
			seen[fullURL] = true
			urls = append(urls, fullURL)
		}
	})

	return urls
}

func resolveURL(baseStr, refStr string) string {
	b, err := url.Parse(baseStr)
	if err != nil {
		return refStr
	}
	r, err := url.Parse(refStr)
	if err != nil {
		return refStr
	}
	return b.ResolveReference(r).String()
}
