"""
Sidecar Solver - Module phụ trợ độc lập (Microservice / CLI helper)
Nhiệm vụ duy nhất: Giải quyết các trang có Cloudflare Turnstile/JS challenge phức tạp
và trả HTML sạch về cho Golang tiếp tục xử lý tốc độ cao.
"""
import sys

def solve(url: str):
    try:
        from curl_cffi import requests
        headers = {
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
            "Accept-Language": "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7",
            "Sec-Ch-Ua": '"Google Chrome";v="120", "Chromium";v="120", "Not.A/Brand";v="24"',
            "Sec-Ch-Ua-Mobile": "?0",
            "Sec-Ch-Ua-Platform": '"Windows"',
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "none",
            "Upgrade-Insecure-Requests": "1",
        }
        resp = requests.get(url, headers=headers, impersonate="chrome120", timeout=15)
        if resp.status_code == 200 and "Just a moment..." not in resp.text:
            print(resp.text)
            sys.exit(0)

        # Nếu domain gốc bị chặn WAF 403, tự động thử mirror sạch
        if "nettruyen" in url:
            for mirror in ["nettruyen.gg", "nettruyenbb.com"]:
                import re
                u = re.sub(r'https?://[^/]+', f'https://{mirror}', url)
                try:
                    r2 = requests.get(u, headers=headers, impersonate="chrome120", timeout=10)
                    if r2.status_code == 200 and "Just a moment..." not in r2.text:
                        print(r2.text)
                        sys.exit(0)
                except Exception:
                    pass

    except Exception as e:
        sys.stderr.write(f"Solver error: {e}\n")

    sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) > 1:
        solve(sys.argv[1])
    else:
        sys.exit(1)
