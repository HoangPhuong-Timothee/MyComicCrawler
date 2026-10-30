import re
from typing import List, Optional
from urllib.parse import urlparse, urljoin
import requests

from ..models import Chapter
from ..network.tls_fetcher import TlsImpersonationFetcher
from ..network.headers_provider import HeadersProvider

class DynamicApiParser:
    """
    Xử lý DẠNG 2 (Mở rộng): AJAX / Dynamic Endpoint Auto-Discovery
    Khi trang web tải chapter bất đồng bộ qua endpoint ẩn (như WordPress Madara,
    ComicService, hoặc REST API), Parser này tự động phát hiện và gọi endpoint để lấy đủ 100% chapters.
    """

    @classmethod
    def try_fetch_dynamic_chapters(cls, base_url: str, html: str, slug: str) -> List[Chapter]:
        chapters: List[Chapter] = []
        fetcher = TlsImpersonationFetcher()

        # 1. Phát hiện hệ thống ComicService (/Comic/Services/ComicService.asmx/ChapterList)
        if "ComicService.asmx" in html or "chapter_list" in html:
            api_url = urljoin(base_url, "/Comic/Services/ComicService.asmx/ChapterList")
            try:
                headers = HeadersProvider.get_html_headers(base_url)
                headers["X-Requested-With"] = "XMLHttpRequest"
                status, text = fetcher.fetch_text(f"{api_url}?slug={slug}", referer=base_url)
                if status == 200:
                    import json
                    res = json.loads(text)
                    items = res.get("data") if isinstance(res, dict) else res
                    if isinstance(items, list):
                        for it in items:
                            num = float(it.get("chapter_num", 0))
                            name = it.get("chapter_name") or f"Chapter {num}"
                            c_slug = it.get("chapter_slug") or f"chuong-{num}"
                            ch_url = urljoin(base_url, f"/truyen-tranh/{slug}/{c_slug}")
                            chapters.append(Chapter(id=ch_url, title=name, url=ch_url, number=num))
                        return chapters
            except Exception:
                pass

        # 2. Phát hiện hệ thống WordPress Madara Manga (/wp-admin/admin-ajax.php)
        if "wp-manga" in html or "admin-ajax.php" in html:
            manga_id_match = re.search(r'data-id="(\d+)"', html)
            if manga_id_match:
                manga_id = manga_id_match.group(1)
                ajax_url = urljoin(base_url, "/wp-admin/admin-ajax.php")
                # Thử gọi post AJAX lấy danh sách chapter
                try:
                    from curl_cffi import requests as c_req
                    res = c_req.post(
                        ajax_url,
                        data={"action": "manga_get_chapters", "manga": manga_id},
                        headers=HeadersProvider.get_html_headers(base_url),
                        impersonate="chrome120",
                        timeout=10
                    )
                    if res.status_code == 200:
                        from bs4 import BeautifulSoup
                        soup = BeautifulSoup(res.text, "html.parser")
                        for a in soup.select("li.wp-manga-chapter a"):
                            href = urljoin(base_url, a["href"])
                            txt = a.get_text(" ", strip=True)
                            chapters.append(Chapter(id=href, title=txt, url=href, number=0.0))
                except Exception:
                    pass

        return chapters

