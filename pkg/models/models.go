package models

// Chapter đại diện cho một chương truyện tranh
type Chapter struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Number float64  `json:"number"`
	Pages  []string `json:"pages"`
}

// Manga đại diện cho thông tin một bộ truyện
type Manga struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	CoverURL    string    `json:"cover_url"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	Chapters    []Chapter `json:"chapters"`
}

// PageData lưu trữ bytes ảnh trong bộ nhớ RAM
type PageData struct {
	Index    int
	Filename string
	Data     []byte
}

