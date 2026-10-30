package network

import (
	"crypto/tls"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type Fetcher struct {
	client     tls_client.HttpClient
	byteClient *nethttp.Client
}

var DefaultFetcher = NewFetcher()

func NewFetcher() *Fetcher {
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithInsecureSkipVerify(),
		tls_client.WithTimeoutSeconds(5),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		// Fallback nếu có lỗi
		client, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger(), tls_client.WithClientProfile(profiles.Chrome_120))
	}

	byteTransport := &nethttp.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
	byteClient := &nethttp.Client{
		Transport: byteTransport,
		Timeout:   10 * time.Second,
	}

	return &Fetcher{
		client:     client,
		byteClient: byteClient,
	}
}

// IsCloudflareChallenge kiểm tra xem phản hồi có phải là màn hình chặn Cloudflare WAF không
func IsCloudflareChallenge(statusCode int, body string) bool {
	if statusCode == 403 || statusCode == 503 {
		lower := strings.ToLower(body)
		return strings.Contains(lower, "just a moment...") ||
			strings.Contains(lower, "challenges.cloudflare.com") ||
			strings.Contains(lower, "cf-turnstile") ||
			strings.Contains(lower, "attention required! | cloudflare")
	}
	return false
}

// MirrorRegistry quản lý các cụm mirror thay thế theo Strategy Pattern (OCP)
type MirrorRegistry struct {
	clusters map[string][]string
}

var DefaultMirrorRegistry = NewMirrorRegistry()

func NewMirrorRegistry() *MirrorRegistry {
	r := &MirrorRegistry{
		clusters: make(map[string][]string),
	}
	// Đăng ký các cụm mạng web truyện lớn (Dễ dàng mở rộng OCP không cần sửa fetcher)
	r.Register("nettruyen", []string{"nettruyen.gg", "nettruyenbb.com", "nettruyenco.vn", "nettruyena.com"})
	r.Register("truyenqq", []string{"truyenqqviet.com", "truyenqq.com.co", "truyenqqto.com"})
	return r
}

func (r *MirrorRegistry) Register(clusterKey string, mirrors []string) {
	r.clusters[strings.ToLower(clusterKey)] = mirrors
}

// GetCandidateMirrors trả về các mirror thay thế cho URL
func (r *MirrorRegistry) GetCandidateMirrors(originalURL string) []string {
	u, err := url.Parse(originalURL)
	if err != nil {
		return nil
	}

	host := strings.ToLower(u.Host)
	var res []string

	for key, mirrors := range r.clusters {
		if strings.Contains(host, key) {
			for _, m := range mirrors {
				if !strings.Contains(host, m) {
					newU := *u
					newU.Host = m
					res = append(res, newU.String())
				}
			}
			break
		}
	}
	return res
}

func (f *Fetcher) Client() tls_client.HttpClient {
	return f.client
}

// FetchHTML tải nội dung HTML với khả năng tự động xử lý bảo mật & Failover
func (f *Fetcher) FetchHTML(targetURL string, referer ...string) (string, string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", "", err
	}

	ref := ""
	if len(referer) > 0 && referer[0] != "" {
		ref = referer[0]
	} else {
		u, _ := url.Parse(targetURL)
		if u != nil {
			ref = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
		}
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", ref)
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="120", "Chromium";v="120", "Not.A/Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := f.client.Do(req)
	var bodyText string
	statusCode := 0

	if err == nil {
		statusCode = resp.StatusCode
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyText = string(bodyBytes)
		if statusCode == 200 && !IsCloudflareChallenge(statusCode, bodyText) {
			return targetURL, bodyText, nil
		}
		// Nếu là mã lỗi client thông thường (404 Not Found, 400 Bad Request, v.v.), không phải do Cloudflare chặn bot (403/503), dừng ngay
		if statusCode == 404 || statusCode == 400 || statusCode == 410 || (statusCode >= 500 && !IsCloudflareChallenge(statusCode, bodyText)) {
			return "", "", fmt.Errorf("HTTP %d error: %s", statusCode, targetURL)
		}
	}

	// Nếu gặp Cloudflare Challenge, 403/503 hoặc timeout kết nối:
	if IsCloudflareChallenge(statusCode, bodyText) || err != nil {
		// Bước 1: Thử gọi Sidecar Solver Python (solver.py) nếu có -> Nhanh nhất (1-2s) & chuẩn xác 100%
		solvedBody, sErr := SolveWithPythonSidecar("solver.py", targetURL)
		if sErr == nil && len(solvedBody) > 0 && !IsCloudflareChallenge(200, solvedBody) {
			return targetURL, solvedBody, nil
		}

		// Bước 2: Thử gọi FlareSolverr (nếu service đang chạy ở port 8191)
		solvedBody, sErr = SolveWithFlareSolverr(targetURL)
		if sErr == nil && len(solvedBody) > 0 && !IsCloudflareChallenge(200, solvedBody) {
			return targetURL, solvedBody, nil
		}

		// Bước 3: Thử các Mirror domain tương đương (nếu thuộc cụm có đăng ký mirror)
		mirrors := DefaultMirrorRegistry.GetCandidateMirrors(targetURL)
		for _, mirrorURL := range mirrors {
			mReq, mErr := http.NewRequest("GET", mirrorURL, nil)
			if mErr == nil {
				mReq.Header = req.Header.Clone()
				mU, _ := url.Parse(mirrorURL)
				if mU != nil {
					mReq.Header.Set("Referer", fmt.Sprintf("%s://%s/", mU.Scheme, mU.Host))
				}
				mResp, doErr := f.client.Do(mReq)
				if doErr == nil {
					mBody, _ := io.ReadAll(mResp.Body)
					mResp.Body.Close()
					mText := string(mBody)
					if mResp.StatusCode == 200 && !IsCloudflareChallenge(mResp.StatusCode, mText) {
						return mirrorURL, mText, nil
					}
				}
			}

			// Nếu mirror cũng bị challenge hoặc handshake timeout, thử giải qua Sidecar
			solvedBody, sErr := SolveWithPythonSidecar("solver.py", mirrorURL)
			if sErr == nil && len(solvedBody) > 0 && !IsCloudflareChallenge(200, solvedBody) {
				return mirrorURL, solvedBody, nil
			}
		}
	}

	if IsCloudflareChallenge(statusCode, bodyText) {
		return "", "", fmt.Errorf("trang web '%s' bị bảo vệ bởi Cloudflare Turnstile/WAF và không có mirror khả dụng", targetURL)
	}

	if err != nil {
		return "", "", fmt.Errorf("lỗi kết nối tới '%s': %w", targetURL, err)
	}

	return "", "", fmt.Errorf("máy chủ phản hồi mã HTTP %d cho url '%s'", statusCode, targetURL)
}

// FetchBytes tải dữ liệu nhị phân (Ảnh) với Referer Contextual Header
// Sử dụng net/http connection pool hiệu năng cao, fallback sang tls_client nếu cần
func (f *Fetcher) FetchBytes(imgURL string, referer string) ([]byte, error) {
	// Cách 1: Thử tải bằng standard Go HTTP Client (Multiplexed Pool, siêu nhanh cho CDN)
	stdReq, err := nethttp.NewRequest("GET", imgURL, nil)
	if err == nil {
		stdReq.Header.Set("User-Agent", DefaultUserAgent)
		stdReq.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		stdReq.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
		if referer != "" {
			stdReq.Header.Set("Referer", referer)
		}
		stdReq.Header.Set("Sec-Fetch-Dest", "image")
		stdReq.Header.Set("Sec-Fetch-Mode", "no-cors")
		stdReq.Header.Set("Sec-Fetch-Site", "cross-site")

		stdResp, stdErr := f.byteClient.Do(stdReq)
		if stdErr == nil {
			if stdResp.StatusCode == nethttp.StatusOK {
				data, readErr := io.ReadAll(stdResp.Body)
				stdResp.Body.Close()
				if readErr == nil && len(data) >= 100 {
					return data, nil
				}
			} else {
				stdResp.Body.Close()
			}
		}
	}

	// Cách 2: Fallback sang tls_client (với Chrome 120 Fingerprint) nếu CDN yêu cầu TLS Handshake đặc biệt
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lỗi tải ảnh: mã HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(data) < 100 {
		return nil, fmt.Errorf("dữ liệu ảnh quá nhỏ (%d bytes)", len(data))
	}

	return data, nil
}
