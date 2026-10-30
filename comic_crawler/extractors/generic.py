import re
from typing import List, Optional
from urllib.parse import urlparse, urljoin
from bs4 import BeautifulSoup

from ..models import Manga, Chapter
from ..network import default_pipeline
from ..parsers import StateDataParser, DynamicApiParser, ImageDecoder
from ..utils import clean_filename
from .base import BaseExtractor

class GenericExtractor(BaseExtractor):
    """
    UNIFIED SMART EXTRACTOR (Thiết kế chuẩn SOLID)
    Tự động phối hợp:
    - NetworkPipeline: Vượt WAF, Cloudflare, TLS Fingerprint, Contextual Referer & Mirror Failover.
    - Parsers: Bóc tách DOM, State Data (Next.js/Nuxt), Dynamic API Endpoint, Base64/Lazy Image Decoder.
    Có khả năng tự động thích ứng với hầu hết mọi website truyện tranh mà không cần code riêng cho từng URL!
    """
    name = "Unified Smart Extractor"

    def can_handle(self, url: str) -> bool:
        return True

    def _extract_number(self, text: str) -> Optional[float]:
        """Tìm số chapter từ chuỗi văn bản (ưu tiên có từ khóa chap/chapter/chương)"""
        if not text:
            return None

        match = re.search(r'(?:chương|chapter|chap|tập|ep|c)\s*[:.\-_]?\s*(\d+(?:\.\d+)?)', text, flags=re.IGNORECASE)
        if match:
            try:
                return float(match.group(1))
            except ValueError:
                pass

        match_url = re.search(r'[-/](?:chap|chapter|chuong|c)-?(\d+(?:\.\d+)?)', text, flags=re.IGNORECASE)
        if match_url:
            try:
                return float(match_url.group(1))
            except ValueError:
                pass

        return None

    def _scan_dom_chapters(self, soup: BeautifulSoup, base_url: str) -> List[Chapter]:
        """Quét tìm chapter từ cấu trúc HTML DOM"""
        containers = soup.select(
            ".works-chapter-list, .list-chapter, .chapter-list, #chapterlist, "
            ".chapters, .listing-chapters_wrap, .wp-manga-chapter, #list-chapter, "
            ".box-list-chapter, .list-item, ul.list-chapters, #examples, .list-wrap, "
            "#nt_listchapter, .version-chap, .chapter-box, #chapter_list"
        )

        candidates = []
        if containers:
            for c in containers:
                candidates.extend(c.find_all("a", href=True))
        else:
            candidates = soup.find_all("a", href=True)

        chapters = []
        seen_urls = set()

        for a in candidates:
            href = a["href"].strip()
            if not href or href.startswith("javascript:") or href.startswith("#"):
                continue

            full_url = urljoin(base_url, href)
            if full_url in seen_urls:
                continue

            a_text = a.get_text(" ", strip=True)

            has_chap_in_href = bool(re.search(r'[-/](?:chap|chapter|chuong|c)[-_/]?\d+', href, flags=re.IGNORECASE))
            has_chap_in_text = bool(re.search(r'(?:chương|chapter|chap|tập)\s*\d+', a_text, flags=re.IGNORECASE))

            if has_chap_in_href or has_chap_in_text:
                seen_urls.add(full_url)
                num = self._extract_number(a_text) or self._extract_number(href) or 0.0
                chap_title = a_text if a_text else f"Chapter {num}"
                chapters.append(Chapter(
                    id=full_url,
                    title=chap_title,
                    url=full_url,
                    number=num
                ))

        return chapters

    def get_manga(self, url: str) -> Manga:
        # 1. Tải HTML thông qua NetworkPipeline (Tự động bypass Cloudflare, WAF, Mirror Failover)
        final_url, html = default_pipeline.fetch_html_smart(url)
        soup = BeautifulSoup(html, "html.parser")
        parsed_url = urlparse(final_url)
        base_url = f"{parsed_url.scheme}://{parsed_url.netloc}"

        # 2. Tìm tiêu đề truyện
        title = ""
        og_title = soup.find("meta", property="og:title")
        if og_title and og_title.get("content"):
            title = og_title["content"].strip()
        if not title:
            h1 = soup.select_one("h1.title-detail, h1.title, h1")
            if h1:
                title = h1.get_text(strip=True)
        if not title and soup.title:
            title = soup.title.get_text(strip=True)

        title = re.sub(
            r'[\-|–|]\s*(NetTruyen|TruyenQQ|BlogTruyen|MangaDex|Webtoon|Truyện Tranh Online|Đọc Truyện).*$',
            '',
            title,
            flags=re.IGNORECASE
        ).strip()

        if not title:
            title = "Comic_" + parsed_url.path.strip('/').split('/')[-1]

        # 3. Tìm cover image
        cover_url = None
        og_image = soup.find("meta", property="og:image")
        if og_image and og_image.get("content"):
            cover_url = urljoin(final_url, og_image["content"])

        # 4. TRÍCH XUẤT CHAPTERS QUA NHIỀU CƠ CHẾ (MULTI-STRATEGY)
        chapters: List[Chapter] = []

        # Strategy A: Dynamic API Discovery (Endpoint tự động cho NetTruyen / Madara / WordPress)
        slug = parsed_url.path.strip('/').split('/')[-1]
        dynamic_chapters = DynamicApiParser.try_fetch_dynamic_chapters(base_url, html, slug)
        if dynamic_chapters:
            chapters = dynamic_chapters

        # Strategy B: DOM Scanning (HTML tĩnh)
        if not chapters:
            chapters = self._scan_dom_chapters(soup, final_url)

        # Strategy C: Nếu là trang đọc đơn lẻ
        if not chapters:
            images = ImageDecoder.extract_image_urls_from_soup(soup, final_url)
            if images:
                chap_num = self._extract_number(final_url) or self._extract_number(title) or 1.0
                chapters.append(Chapter(
                    id="chap_1",
                    title=title,
                    url=final_url,
                    number=chap_num,
                    pages=images
                ))

        # Sắp xếp chapters tăng dần theo số thứ tự
        chapters.sort(key=lambda x: x.number)

        return Manga(
            title=title,
            url=final_url,
            cover_url=cover_url,
            chapters=chapters
        )

    def get_chapter_images(self, chapter_url: str) -> List[str]:
        final_url, html = default_pipeline.fetch_html_smart(chapter_url, referer=chapter_url)
        soup = BeautifulSoup(html, "html.parser")
        return ImageDecoder.extract_image_urls_from_soup(soup, final_url)

    def get_referer(self, url: str) -> str:
        parsed = urlparse(url)
        return f"{parsed.scheme}://{parsed.netloc}/"
