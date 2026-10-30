from typing import Optional, Tuple
from urllib.parse import urlparse
import requests

from .base_fetcher import BaseFetcher
from .headers_provider import HeadersProvider

try:
    from curl_cffi import requests as curl_requests
    HAS_CURL_CFFI = True
except ImportError:
    HAS_CURL_CFFI = False

class TlsImpersonationFetcher(BaseFetcher):
    """
    Xử lý DẠNG 1: TLS Fingerprinting / Bot Detection Bypass
    Sử dụng curl_cffi để giả lập 100% chữ ký TLS (JA3/JA4, HTTP/2, Client Hello)
    của Google Chrome 120/124.
    """
    name = "TlsImpersonationFetcher"

    def fetch_text(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[int, str]:
        headers = HeadersProvider.get_html_headers(url, referer)
        if HAS_CURL_CFFI:
            # Thử với các browser fingerprint phổ biến
            for browser_profile in ["chrome120", "chrome124", "edge101"]:
                try:
                    resp = curl_requests.get(
                        url,
                        headers=headers,
                        timeout=timeout,
                        impersonate=browser_profile
                    )
                    return resp.status_code, resp.text
                except Exception:
                    continue

        # Fallback requests tiêu chuẩn
        try:
            s = requests.Session()
            s.headers.update(headers)
            resp = s.get(url, timeout=timeout)
            return resp.status_code, resp.text
        except requests.HTTPError as e:
            code = e.response.status_code if e.response else 500
            return code, ""
        except Exception:
            return 500, ""

    def fetch_bytes(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[int, Optional[bytes], Optional[str]]:
        headers = HeadersProvider.get_image_headers(url, referer)
        if HAS_CURL_CFFI:
            try:
                resp = curl_requests.get(
                    url,
                    headers=headers,
                    timeout=timeout,
                    impersonate="chrome120"
                )
                if resp.status_code == 200:
                    return 200, resp.content, resp.headers.get("content-type")
                return resp.status_code, None, None
            except Exception:
                pass

        # Fallback requests
        try:
            s = requests.Session()
            s.headers.update(headers)
            resp = s.get(url, timeout=timeout)
            if resp.status_code == 200:
                return 200, resp.content, resp.headers.get("content-type")
            return resp.status_code, None, None
        except Exception:
            return 500, None, None

