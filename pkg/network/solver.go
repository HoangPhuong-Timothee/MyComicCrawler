package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FlareSolverrRequest cấu trúc gửi tới FlareSolverr API
type FlareSolverrRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
}

type FlareSolverrResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	Solution       struct {
		URL      string `json:"url"`
		Status   int    `json:"status"`
		Response string `json:"response"`
		Cookies  []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	} `json:"solution"`
}

// SolveWithFlareSolverr gửi yêu cầu sang FlareSolverr (mặc định http://localhost:8191/v1)
func SolveWithFlareSolverr(targetURL string, flareURL ...string) (string, error) {
	endpoint := "http://localhost:8191/v1"
	if len(flareURL) > 0 && flareURL[0] != "" {
		endpoint = flareURL[0]
	}

	reqBody := FlareSolverrRequest{
		Cmd:        "request.get",
		URL:        targetURL,
		MaxTimeout: 30000,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 35 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("không thể kết nối tới FlareSolverr (%s): %w", endpoint, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var solResp FlareSolverrResponse
	if err := json.Unmarshal(bodyBytes, &solResp); err != nil {
		return "", err
	}

	if solResp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr trả về lỗi: %s", solResp.Message)
	}

	return solResp.Solution.Response, nil
}

// SolveWithPythonSidecar gọi script phụ trợ bằng python (nếu có cài python)
func SolveWithPythonSidecar(scriptPath string, targetURL string) (string, error) {
	resolvedScript := scriptPath
	if !filepath.IsAbs(scriptPath) {
		if exePath, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exePath), scriptPath)
			if _, err := os.Stat(candidate); err == nil {
				resolvedScript = candidate
			}
		}
	}

	cmd := exec.Command("python", resolvedScript, targetURL)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("sidecar solver thất bại: %w (stderr: %s)", err, errBuf.String())
	}

	outStr := outBuf.String()
	if strings.Contains(outStr, "Just a moment...") {
		return "", fmt.Errorf("sidecar vẫn chưa giải được challenge")
	}

	return outStr, nil
}
