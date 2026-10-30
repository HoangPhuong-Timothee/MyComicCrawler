from abc import ABC, abstractmethod
from typing import List, Tuple
from ..models import Manga, Chapter

class BaseExtractor(ABC):
    name: str = "Base"

    @abstractmethod
    def can_handle(self, url: str) -> bool:
        """Kiểm tra xem extractor có hỗ trợ URL này hay không"""
        pass

    @abstractmethod
    def get_manga(self, url: str) -> Manga:
        """Lấy thông tin bộ truyện và danh sách các chapter"""
        pass

    @abstractmethod
    def get_chapter_images(self, chapter_url: str) -> List[str]:
        """Lấy danh sách link ảnh của một chapter"""
        pass

    def get_referer(self, url: str) -> str:
        """Referer header mặc định khi tải ảnh"""
        from urllib.parse import urlparse
        parsed = urlparse(url)
        return f"{parsed.scheme}://{parsed.netloc}/"

