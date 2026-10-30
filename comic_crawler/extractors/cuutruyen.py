import re
import requests
from typing import List
from ..models import Manga, Chapter
from .base import BaseExtractor

class CuuTruyenExtractor(BaseExtractor):
    name = "CuuTruyen"

    def can_handle(self, url: str) -> bool:
        return "cuutruyen.net" in url

    def _extract_id(self, url: str) -> str:
        # cuutruyen.net/mangas/123 hoặc cuutruyen.net/mangas/123-ten-truyen
        match = re.search(r'mangas/(\d+)', url)
        if match:
            return match.group(1)
        # Hoặc link chapter: cuutruyen.net/mangas/123/chapters/456
        match_chap = re.search(r'chapters/(\d+)', url)
        if match_chap:
            chap_id = match_chap.group(1)
            # lấy chapter info để tìm manga_id
            resp = requests.get(f"https://cuutruyen.net/api/v2/chapters/{chap_id}", timeout=15).json()
            return str(resp.get("data", {}).get("manga_id"))
        return ""

    def get_manga(self, url: str) -> Manga:
        manga_id = self._extract_id(url)
        if not manga_id:
            # Fallback sang Generic
            from .generic import GenericExtractor
            return GenericExtractor().get_manga(url)

        from ..utils import get_session
        session = get_session(referer="https://cuutruyen.net/")
        resp = session.get(f"https://cuutruyen.net/api/v2/mangas/{manga_id}", timeout=25).json()
        data = resp.get("data", {})
        title = data.get("name") or "CuuTruyen Manga"

        # Lấy danh sách chapters
        ch_resp = session.get(f"https://cuutruyen.net/api/v2/mangas/{manga_id}/chapters", timeout=25).json()
        ch_list = ch_resp.get("data", [])

        chapters = []
        for ch in ch_list:
            ch_num_raw = ch.get("number", 0)
            try:
                ch_num = float(ch_num_raw)
            except (ValueError, TypeError):
                ch_num = 0.0

            name = ch.get("name") or ""
            ch_title = f"Chương {ch_num_raw}"
            if name:
                ch_title += f" - {name}"

            ch_id = str(ch.get("id"))
            chapters.append(Chapter(
                id=ch_id,
                title=ch_title,
                url=f"https://cuutruyen.net/api/v2/chapters/{ch_id}",
                number=ch_num
            ))

        chapters.sort(key=lambda x: x.number)

        return Manga(
            title=title,
            url=url,
            chapters=chapters
        )

    def get_chapter_images(self, chapter_url: str) -> List[str]:
        # Nếu chapter_url là api link hoặc web link
        match = re.search(r'chapters/(\d+)', chapter_url)
        if match:
            chap_id = match.group(1)
            api_url = f"https://cuutruyen.net/api/v2/chapters/{chap_id}"
        else:
            api_url = chapter_url

        from ..utils import get_session
        session = get_session(referer="https://cuutruyen.net/")
        for attempt in range(3):
            try:
                resp = session.get(api_url, timeout=25).json()
                pages = resp.get("data", {}).get("pages", [])
                return [p.get("image_url") for p in pages if p.get("image_url")]
            except Exception:
                if attempt == 2:
                    raise
                import time
                time.sleep(1)
        return []
