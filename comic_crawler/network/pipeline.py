from typing import Optional, Tuple
from urllib.parse import urlparse

from .base_fetcher import BaseFetcher
from .tls_fetcher import TlsImpersonationFetcher
from .mirror_resolver import MirrorResolver

class NetworkPipeline:
    """
    TẦNG NETWORK PIPELINE (Áp dụng Chain of Responsibility & Strategy Pattern)
    Điều phối các Fetcher để tự động nhận diện và vượt qua các cơ chế bảo mật:
    - Nhận diện Cloudflare WAF / Just a moment...
    - Tự động chuyển đổi TLS Fingerprint (Chrome 120/124)
    - Tự động Failover sang Mirror khi domain gốc bị khóa cứng
    """

    def __init__(self, primary_fetcher: Optional[BaseFetcher] = None):
        self.primary_fetcher = primary_fetcher or TlsImpersonationFetcher()

    def is_cloudflare_challenge(self, status_code: int, text: str) -> bool:
        """Kiểm tra xem response có phải là màn hình chặn Cloudflare WAF không"""
        if status_code in (403, 503):
            lower_text = text.lower()
            return (
                "just a moment..." in lower_text or
                "challenges.cloudflare.com" in lower_text or
                "cf-turnstile" in lower_text or
                "attention required! | cloudflare" in lower_text
            )
        return False

    def fetch_html_smart(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[str, str]:
        """
        Tải HTML trang web thông minh:
        - Tự động vượt Cloudflare
        - Nếu domain bị lock, tự failover sang Mirror
        Trả về: (final_working_url, html_content)
        """
        # 1. Thử tải với Primary Fetcher (TLS Impersonation)
        status_code, text = self.primary_fetcher.fetch_text(url, referer=referer, timeout=timeout)

        if status_code == 200 and not self.is_cloudflare_challenge(status_code, text):
            return url, text

        # 2. Nếu bị Cloudflare WAF hoặc 403, thử các Candidate Mirrors
        if self.is_cloudflare_challenge(status_code, text) or status_code in (403, 503):
            mirrors = MirrorResolver.get_candidate_mirrors(url)
            for mirror_url in mirrors:
                m_status, m_text = self.primary_fetcher.fetch_text(mirror_url, referer=referer, timeout=8)
                if m_status == 200 and not self.is_cloudflare_challenge(m_status, m_text):
                    return mirror_url, m_text

        # 3. Nếu vẫn không vượt được, trả về lỗi chi tiết
        if self.is_cloudflare_challenge(status_code, text):
            raise ConnectionError(
                f"Trang web '{url}' đang bật chế độ bảo mật Cloudflare Turnstile/WAF "
                f"và không có mirror thay thế nào khả dụng. Vui lòng thử lại sau hoặc dùng nguồn khác."
            )

        raise ConnectionError(f"Không thể kết nối đến URL '{url}' (Mã phản hồi HTTP: {status_code})")

    def fetch_image_smart(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[Optional[bytes], Optional[str]]:
        """Tải dữ liệu ảnh với Contextual Headers chống Anti-Leech CDN"""
        status_code, content, ctype = self.primary_fetcher.fetch_bytes(url, referer=referer, timeout=timeout)
        if status_code == 200 and content and len(content) > 500:
            return content, ctype
        return None, None

