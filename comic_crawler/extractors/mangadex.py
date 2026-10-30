import re
import requests
from typing import List
from ..models import Manga, Chapter
from .base import BaseExtractor

class MangaDexExtractor(BaseExtractor):
    name = "MangaDex"

    def can_handle(self, url: str) -> bool:
        return "mangadex.org" in url

    def _extract_manga_id(self, url: str) -> str:
        match = re.search(r'title/([0-9a-fA-F\-]+)', url)
        if match:
            return match.group(1)
        # Hoặc chapter URL: /chapter/xxx
        match_chap = re.search(r'chapter/([0-9a-fA-F\-]+)', url)
        if match_chap:
            # Lấy chapter info để tìm manga id
            chap_id = match_chap.group(1)
            resp = requests.get(f"https://api.mangadex.org/chapter/{chap_id}", timeout=15).json()
            for rel in resp.get("data", {}).get("relationships", []):
                if rel.get("type") == "manga":
                    return rel.get("id")
        return ""

    def get_manga(self, url: str) -> Manga:
        manga_id = self._extract_manga_id(url)
        if not manga_id:
            raise ValueError(f"Không thể tìm thấy Manga ID từ MangaDex URL: {url}")

        # Lấy chi tiết manga
        resp = requests.get(f"https://api.mangadex.org/manga/{manga_id}", timeout=15).json()
        data = resp.get("data", {})
        attr = data.get("attributes", {})
        
        # Tiêu đề ưu tiên: vi -> en -> ja-ro -> bất kỳ
        title_dict = attr.get("title", {})
        title = title_dict.get("vi") or title_dict.get("en") or next(iter(title_dict.values()), "MangaDex Comic")

        # Lấy danh sách chapters (hỗ trợ cả tiếng Việt và tiếng Anh)
        # Thử lấy tiếng Việt trước, nếu không có lấy tiếng Anh
        feed_url = f"https://api.mangadex.org/manga/{manga_id}/feed"
        params = {
            "translatedLanguage[]": ["vi", "en"],
            "order[chapter]": "asc",
            "limit": 500
        }
        feed_resp = requests.get(feed_url, params=params, timeout=20).json()
        chapters_data = feed_resp.get("data", [])

        # Nếu không có vi/en, lấy tất cả ngôn ngữ
        if not chapters_data:
            params.pop("translatedLanguage[]", None)
            feed_resp = requests.get(feed_url, params=params, timeout=20).json()
            chapters_data = feed_resp.get("data", [])

        chapters = []
        for ch in chapters_data:
            ch_attr = ch.get("attributes", {})
            # Bỏ qua các chapter là link ngoài (MangaPlus) hoặc 0 trang
            if ch_attr.get("externalUrl") or ch_attr.get("pages", 1) == 0:
                continue

            ch_num_str = ch_attr.get("chapter") or "0"
            try:
                ch_num = float(ch_num_str)
            except ValueError:
                ch_num = 0.0

            ch_title = ch_attr.get("title") or f"Chapter {ch_num_str}"
            ch_id = ch.get("id")
            ch_url = f"https://mangadex.org/chapter/{ch_id}"
            
            # Đính kèm ngôn ngữ vào title nếu có nhiều thứ tiếng
            lang = ch_attr.get("translatedLanguage", "")
            full_title = f"[{lang.upper()}] {ch_title}" if lang else ch_title

            chapters.append(Chapter(
                id=ch_id,
                title=full_title,
                url=ch_url,
                number=ch_num
            ))

        return Manga(
            title=title,
            url=url,
            chapters=chapters
        )

    def get_chapter_images(self, chapter_url: str) -> List[str]:
        match = re.search(r'chapter/([0-9a-fA-F\-]+)', chapter_url)
        chap_id = match.group(1) if match else chapter_url

        # Gọi server at-home API của MangaDex
        resp = requests.get(f"https://api.mangadex.org/at-home/server/{chap_id}", timeout=15).json()
        base_url = resp.get("baseUrl")
        chapter_hash = resp.get("chapter", {}).get("hash")
        files = resp.get("chapter", {}).get("data", [])

        images = []
        for filename in files:
            images.append(f"{base_url}/data/{chapter_hash}/{filename}")
        return images
