package parsers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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

	// 3. Trích xuất Chapters theo Strategy Pipeline (Chuẩn SOLID - OCP)
	slug := extractSlug(u.Path)

	// Strategy A: Quét Generic DOM Selector trước (Siêu nhanh 1ms trong RAM cho 90% trang)
	chapters := scanDOMChapters(doc, baseURL, finalURL)

	// Strategy B: Nếu DOM chưa có chapter hoặc web dùng Ajax phân trang ẩn (như NetTruyen), thử Dynamic API
	hasMoreHidden := doc.Find("#nt_listchapter, .hidden_chapters, .show-more, a#show-more, .btn-show-more, a[onclick*='ChapterList']").Length() > 0
	if len(chapters) == 0 || (len(chapters) <= 25 && hasMoreHidden) {
		apiChapters := tryDynamicAPIs(baseURL, finalURL, slug)
		if len(apiChapters) > len(chapters) {
			chapters = apiChapters
		}
	}

	// Strategy C: Bóc tách Script State (Next.js / Nuxt SPA)
	if len(chapters) == 0 {
		chapters = tryExtractScriptState(doc, baseURL, slug)
	}

	// Strategy D: Fallback URL đơn lẻ là trang đọc của 1 chương
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
				ID:     finalURL,
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

// ExtractChapterImages bóc tách danh sách link ảnh của 1 chương (kèm retry)
func (e *Extractor) ExtractChapterImages(chapterURL string) ([]string, error) {
	var finalURL, html string
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		finalURL, html, err = e.fetcher.FetchHTML(chapterURL, chapterURL)
		if err == nil && len(html) > 0 {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 800 * time.Millisecond)
	}
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
	prefixes := []string{"truyen-tranh", "manga", "series", "comic", "truyen", "read"}
	for i, p := range parts {
		for _, prefix := range prefixes {
			if strings.EqualFold(p, prefix) && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// tryDynamicAPIs thử khám phá danh sách chương qua các mẫu endpoint AJAX/API phổ biến
func tryDynamicAPIs(baseURL, referer, slug string) []models.Chapter {
	if slug == "" {
		return nil
	}

	apiPatterns := []string{
		"%s/Comic/Services/ComicService.asmx/ChapterList?slug=%s",
		"%s/ajax/manga/%s/chapters",
		"%s/api/manga/%s/chapters",
		"%s/api/chapters?slug=%s",
	}

	for _, pattern := range apiPatterns {
		var apiURL string
		if strings.Count(pattern, "%s") == 2 {
			apiURL = fmt.Sprintf(pattern, baseURL, url.QueryEscape(slug))
		} else {
			apiURL = fmt.Sprintf(pattern, baseURL, slug)
		}

		_, html, err := network.DefaultFetcher.FetchHTML(apiURL, referer)
		if err != nil || len(html) == 0 {
			continue
		}

		type apiItem struct {
			ChapterNum  float64 `json:"chapter_num"`
			ChapterName string  `json:"chapter_name"`
			ChapterSlug string  `json:"chapter_slug"`
			Number      float64 `json:"number"`
			Name        string  `json:"name"`
			URL         string  `json:"url"`
		}
		type apiResp struct {
			Data     []apiItem `json:"data"`
			Chapters []apiItem `json:"chapters"`
		}

		var apiResult apiResp
		if err := json.Unmarshal([]byte(html), &apiResult); err == nil {
			items := apiResult.Data
			if len(items) == 0 {
				items = apiResult.Chapters
			}
			if len(items) > 0 {
				var chapters []models.Chapter
				for _, it := range items {
					num := it.ChapterNum
					if num == 0 {
						num = it.Number
					}
					name := it.ChapterName
					if name == "" {
						name = it.Name
					}
					if name == "" {
						name = fmt.Sprintf("Chapter %g", num)
					}

					chURL := it.URL
					if chURL == "" {
						cSlug := it.ChapterSlug
						if cSlug == "" {
							cSlug = fmt.Sprintf("chuong-%g", num)
						}
						chURL = fmt.Sprintf("%s/truyen-tranh/%s/%s", baseURL, slug, cSlug)
					} else {
						chURL = resolveURL(baseURL, chURL)
					}

					chapters = append(chapters, models.Chapter{
						ID:     chURL,
						Title:  name,
						URL:    chURL,
						Number: num,
					})
				}
				return chapters
			}
		}
	}
	return nil
}

// tryExtractScriptState bóc tách state JSON nhúng trong <script> (Next.js __NEXT_DATA__, Nuxt, SPA state)
func tryExtractScriptState(doc *goquery.Document, baseURL, slug string) []models.Chapter {
	var chapters []models.Chapter

	doc.Find("script#__NEXT_DATA__").Each(func(i int, s *goquery.Selection) {
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return
		}
		var root map[string]interface{}
		if err := json.Unmarshal([]byte(jsonText), &root); err == nil {
			if chaps := searchChaptersRecursive(root, baseURL, slug); len(chaps) > 0 {
				chapters = chaps
			}
		}
	})

	return chapters
}

func searchChaptersRecursive(data interface{}, baseURL, slug string) []models.Chapter {
	switch val := data.(type) {
	case map[string]interface{}:
		for k, v := range val {
			if strings.Contains(strings.ToLower(k), "chapter") {
				if list, ok := v.([]interface{}); ok && len(list) > 0 {
					var res []models.Chapter
					for _, item := range list {
						if m, ok := item.(map[string]interface{}); ok {
							num := 0.0
							if n, ok := m["chapter_num"].(float64); ok {
								num = n
							} else if n, ok := m["number"].(float64); ok {
								num = n
							}
							title := fmt.Sprintf("Chapter %g", num)
							if t, ok := m["chapter_name"].(string); ok && t != "" {
								title = t
							}
							u := ""
							if link, ok := m["url"].(string); ok {
								u = resolveURL(baseURL, link)
							} else {
								u = fmt.Sprintf("%s/manga/%s/chapter-%g", baseURL, slug, num)
							}
							res = append(res, models.Chapter{
								ID:     u,
								Title:  title,
								URL:    u,
								Number: num,
							})
						}
					}
					if len(res) > 0 {
						return res
					}
				}
			}
			if chaps := searchChaptersRecursive(v, baseURL, slug); len(chaps) > 0 {
				return chaps
			}
		}
	case []interface{}:
		for _, v := range val {
			if chaps := searchChaptersRecursive(v, baseURL, slug); len(chaps) > 0 {
				return chaps
			}
		}
	}
	return nil
}

func scanDOMChapters(doc *goquery.Document, baseURL, currentURL string) []models.Chapter {
	var chapters []models.Chapter
	seen := make(map[string]bool)

	selector := ".works-chapter-list a, .list-chapter a, .chapter-list a, #chapterlist a, " +
		".chapters a, .listing-chapters_wrap a, .wp-manga-chapter a, #list-chapter a, " +
		".box-list-chapter a, ul.list-chapters a, #examples a, .list-wrap a, #nt_listchapter a, #chapter_list a, " +
		".reading-list a, .list-chapters a, .item-name a, .chapter-item a, div.item a"

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
