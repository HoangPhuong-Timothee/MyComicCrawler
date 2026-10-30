from typing import List
from .base import BaseExtractor
from .mangadex import MangaDexExtractor
from .cuutruyen import CuuTruyenExtractor
from .generic import GenericExtractor

# EXTRACTORS được tổ chức theo kiến trúc SOLID:
# 1. Các nguồn Open API chính thống (MangaDex, CuuTruyen)
# 2. Unified Smart Extractor xử lý tự động toàn bộ website khác dựa trên cơ chế bảo mật (Pipeline & Parsers)
EXTRACTORS: List[BaseExtractor] = [
    MangaDexExtractor(),
    CuuTruyenExtractor(),
    GenericExtractor(),
]

def get_extractor(url: str) -> BaseExtractor:
    """Tìm extractor phù hợp nhất cho URL đã cho"""
    for extractor in EXTRACTORS:
        if extractor.can_handle(url):
            return extractor
    return GenericExtractor()
