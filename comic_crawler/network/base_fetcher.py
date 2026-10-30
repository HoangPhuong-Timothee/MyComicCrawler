from abc import ABC, abstractmethod
from typing import Optional, Dict, Tuple

class BaseFetcher(ABC):
    """
    Interface chuẩn cho mọi Fetcher theo nguyên lý SOLID (DIP & ISP).
    Mỗi Fetcher chịu trách nhiệm cho một cơ chế tải / vượt bảo mật cụ thể (SRP).
    """
    name: str = "BaseFetcher"

    @abstractmethod
    def fetch_text(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[int, str]:
        """Tải nội dung trang web dưới dạng văn bản (HTML/JSON), trả về (status_code, text)"""
        pass

    @abstractmethod
    def fetch_bytes(self, url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[int, Optional[bytes], Optional[str]]:
        """Tải dữ liệu nhị phân (Ảnh), trả về (status_code, bytes_data, content_type)"""
        pass

