import re
import base64
from typing import List, Tuple, Optional
from urllib.parse import urljoin
from bs4 import BeautifulSoup

class ImageDecoder:
    """
    Xử lý DẠNG 4: Obfuscated / Base64 Data URI & Lazy-load Images
    - Tự động bóc tách link ảnh từ các thuộc tính bảo vệ/lazy-load
    - Giải mã trực tiếp Base64 Data URI thành bytes
    - Lọc bỏ các ảnh tracking/banner/icon rác
    """

    BAD_KEYWORDS = [
        "logo", "banner", "facebook", "discord", "donate", "icon",
        "advert", "loading", "placeholder", "button", "avatar", "tracking", "pixel"
    ]

    LAZY_ATTRIBUTES = [
        "data-original", "data-src", "data-cdn", "data-lazy-src",
        "data-url", "data-sv1", "data-sv2", "data-echo", "src"
    ]

    @classmethod
    def is_base64_data_uri(cls, src: str) -> bool:
        return src.startswith("data:image/")

    @classmethod
    def decode_base64_image(cls, data_uri: str) -> Optional[Tuple[bytes, str]]:
        """Giải mã data:image/png;base64,.... thành (bytes, extension)"""
        try:
            header, encoded = data_uri.split(",", 1)
            ext = ".png" if "png" in header else ".jpg"
            data = base64.b64decode(encoded)
            return data, ext
        except Exception:
            return None

    @classmethod
    def extract_image_urls_from_soup(cls, soup: BeautifulSoup, base_url: str) -> List[str]:
        """Trích xuất và chuẩn hóa tất cả các link ảnh hợp lệ từ HTML của chapter"""
        # Ưu tiên các container đọc truyện phổ biến
        containers = soup.select(
            ".story-see-content, .reading-detail, .page-chapter, .chapter-content, "
            ".reading-content, #readerarea, .vung-doc, #viewer, .image-container, "
            ".entry-content, .box-read-content, #chapter-content, .read-content"
        )

        img_tags = []
        if containers:
            for c in containers:
                img_tags.extend(c.find_all("img"))
        else:
            divs = soup.find_all("div", class_=re.compile(r'(read|chapter|viewer|story-see)', re.IGNORECASE))
            for d in divs:
                img_tags.extend(d.find_all("img"))

        if not img_tags:
            img_tags = soup.find_all("img")

        urls = []
        seen = set()

        for img in img_tags:
            src = ""
            for attr in cls.LAZY_ATTRIBUTES:
                val = img.get(attr)
                if val and val.strip():
                    src = val.strip()
                    break

            if not src:
                continue

            # Xử lý Base64 Data URI
            if cls.is_base64_data_uri(src):
                urls.append(src)
                continue

            # Bù giao thức //domain.com/1.jpg
            if src.startswith("//"):
                src = "https:" + src

            full_img_url = urljoin(base_url, src)
            lower_url = full_img_url.lower()

            if any(bad in lower_url for bad in cls.BAD_KEYWORDS):
                continue

            if full_img_url not in seen:
                seen.add(full_img_url)
                urls.append(full_img_url)

        return urls

