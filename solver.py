"""
Sidecar Solver - Generic Microservice / CLI Helper
Nhiệm vụ duy nhất: Giải quyết các trang có Cloudflare WAF / Turnstile / JS Challenge
bằng TLS Impersonation (BoringSSL Chrome 120), trả HTML sạch về cho Golang tiếp tục xử lý tốc độ cao.
Hoàn toàn độc lập, không hardcode bất kỳ domain hay URL cụ thể nào (Chuẩn SOLID - SRP).
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
        # Thử với Chrome 120 impersonation
        resp = requests.get(url, headers=headers, impersonate="chrome120", timeout=15)
        if resp.status_code == 200 and "Just a moment..." not in resp.text:
            print(resp.text)
            sys.exit(0)

        # Fallback thử với Chrome 124 nếu Chrome 120 chưa qua được
        resp2 = requests.get(url, headers=headers, impersonate="chrome124", timeout=15)
        if resp2.status_code == 200 and "Just a moment..." not in resp2.text:
            print(resp2.text)
            sys.exit(0)

    except Exception as e:
        sys.stderr.write(f"Solver error: {e}\n")

    sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) > 1:
        solve(sys.argv[1])
    else:
        sys.exit(1)
