import json
import re
from typing import List, Dict, Any, Optional
from bs4 import BeautifulSoup
from ..models import Chapter

class StateDataParser:
    """
    Xử lý DẠNG 2: Client-side Rendering (SPA / Next.js / Nuxt / Embedded State)
    Nhiều trang web truyện tranh hiện đại render bằng Next.js hoặc Vue,
    dữ liệu chapters và images nằm sẵn trong thẻ <script id="__NEXT_DATA__"> hoặc script biến toàn cục.
    Parser này trích xuất trực tiếp dữ liệu gốc với độ chính xác tuyệt đối.
    """

    @classmethod
    def extract_next_data(cls, soup: BeautifulSoup) -> Optional[Dict[str, Any]]:
        """Trích xuất Next.js __NEXT_DATA__ JSON"""
        script = soup.find("script", id="__NEXT_DATA__")
        if script and script.string:
            try:
                return json.loads(script.string)
            except Exception:
                pass
        return None

    @classmethod
    def extract_js_variables(cls, html: str) -> Dict[str, Any]:
        """Trích xuất các biến JavaScript mảng chapters hoặc images nhúng trong code"""
        results = {}

        # 1. Tìm mảng chapters: var chapters = [...]; hoặc const chapter_list = [...];
        chap_match = re.search(
            r'(?:var|let|const)\s+(?:chapters|chapterList|chapter_list|listChapters)\s*=\s*(\[.*?\]);',
            html,
            flags=re.DOTALL
        )
        if chap_match:
            try:
                results["chapters"] = json.loads(chap_match.group(1))
            except Exception:
                pass

        # 2. Tìm mảng images: var pages = [...]; hoặc const images = [...];
        img_match = re.search(
            r'(?:var|let|const)\s+(?:pages|images|chapter_images|page_images)\s*=\s*(\[.*?\]);',
            html,
            flags=re.DOTALL
        )
        if img_match:
            try:
                results["images"] = json.loads(img_match.group(1))
            except Exception:
                pass

        return results

