import os
import re
from pathlib import Path
from urllib.parse import urlparse
from typing import Optional, Tuple, Union
import requests

try:
    from curl_cffi import requests as curl_requests
    HAS_CURL_CFFI = True
except ImportError:
    HAS_CURL_CFFI = False

DEFAULT_COMIC_DIR = Path(r"C:\Users\ROSA\workspace\comic")

# Headers giả lập Chrome trên Windows để không bị chặn
DEFAULT_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
    "Accept-Language": "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7",
    "Cache-Control": "no-cache",
    "Pragma": "no-cache",
}

def resolve_output_dir(user_input: Optional[Union[str, Path]] = None) -> Path:
    """
    Xác định chính xác thư mục lưu truyện:
    - Nếu để trống / None: mặc định C:\\Users\\ROSA\\workspace\\comic
    - Nếu đã là đối tượng Path: xử lý trực tiếp
    - Nếu nhập tên tương đối (vd: '20centuryboys'): lưu vào C:\\Users\\ROSA\\workspace\\comic\\20centuryboys
    - Nếu nhập đường dẫn tuyệt đối (vd: 'D:\\Truyen' hoặc 'C:\\Users\\...'): lưu đúng đường dẫn đó
    Tuyệt đối không lưu vào source code!
    """
    if user_input is None:
        return DEFAULT_COMIC_DIR

    if isinstance(user_input, Path):
        target_path = user_input
    else:
        cleaned_input = str(user_input).strip().strip('"').strip("'")
        if not cleaned_input:
            return DEFAULT_COMIC_DIR
        target_path = Path(cleaned_input)

    if target_path.is_absolute():
        return target_path.resolve()
    else:
        # Nếu nhập tên thư mục con (vd: 20centuryboys) -> nối vào DEFAULT_COMIC_DIR
        return (DEFAULT_COMIC_DIR / target_path).resolve()

def clean_filename(name: str, max_length: int = 150) -> str:
    """Làm sạch tên file/thư mục để tương thích cả Windows và Android filesystem"""
    if not name:
        return "unnamed"
    # Thay thế các ký tự cấm: \ / : * ? " < > |
    cleaned = re.sub(r'[\\/*?:"<>|]', '_', name)
    # Bỏ các khoảng trắng thừa hoặc ký tự điều khiển
    cleaned = re.sub(r'[\r\n\t]+', ' ', cleaned)
    cleaned = re.sub(r'\s+', ' ', cleaned).strip('. ')
    if not cleaned:
        cleaned = "unnamed"
    return cleaned[:max_length]

def format_chapter_name(number: float, title: str = "") -> str:
    """
    Format tên chapter chuẩn cho máy đọc sách:
    Ví dụ: 'Chapter 001', 'Chapter 012.5 - Tiêu đề', ...
    Zero-padding 3 chữ số để máy đọc sách (Mihon/Moon+) sort đúng thứ tự 001 -> 999
    """
    num_str = f"{number:07.2f}".rstrip('0').rstrip('.')
    if '.' in num_str:
        integer_part, dec_part = num_str.split('.')
        padded = f"{int(integer_part):03d}.{dec_part}"
    else:
        padded = f"{int(number):03d}"

    base = f"Chapter {padded}"
    if title:
        clean_title = re.sub(r'^(Chương|Chapter|Chap)\s*\d+([\s:\-_]+)?', '', title, flags=re.IGNORECASE).strip()
        if clean_title:
            base = f"{base} - {clean_title}"
    return clean_filename(base)

def format_page_filename(page_num: int, ext: str = ".jpg") -> str:
    """Đánh số trang 3 chữ số: 001.jpg, 002.jpg để không bị lộn thứ tự"""
    if not ext.startswith('.'):
        ext = f".{ext}"
    return f"{page_num:03d}{ext.lower()}"

def detect_image_extension(content_type: Optional[str], url: str, first_bytes: bytes = b"") -> str:
    """Xác định đuôi ảnh dựa trên Content-Type, Magic bytes hoặc URL"""
    if first_bytes.startswith(b"\xff\xd8\xff"):
        return ".jpg"
    elif first_bytes.startswith(b"\x89PNG\r\n\x1a\n"):
        return ".png"
    elif first_bytes.startswith(b"RIFF") and b"WEBP" in first_bytes[:16]:
        return ".webp"
    elif first_bytes.startswith(b"GIF8"):
        return ".gif"

    if content_type:
        ct = content_type.lower()
        if "jpeg" in ct or "jpg" in ct:
            return ".jpg"
        if "png" in ct:
            return ".png"
        if "webp" in ct:
            return ".webp"
        if "gif" in ct:
            return ".gif"

    parsed = urlparse(url).path.lower()
    for ext in [".jpg", ".jpeg", ".png", ".webp", ".gif"]:
        if parsed.endswith(ext):
            return ".jpg" if ext == ".jpeg" else ext

    return ".jpg"

def fetch_html(url: str, referer: Optional[str] = None, timeout: int = 25) -> str:
    """
    Tải HTML trang web, tự động giả lập TLS browser fingerprint (Chrome 120)
    để vượt qua Cloudflare và chống SSL EOF error
    """
    headers = DEFAULT_HEADERS.copy()
    if referer:
        headers["Referer"] = referer

    if HAS_CURL_CFFI:
        try:
            resp = curl_requests.get(url, headers=headers, timeout=timeout, impersonate="chrome120")
            if resp.status_code == 200:
                return resp.text
        except Exception:
            pass

    # Fallback requests
    session = requests.Session()
    session.headers.update(headers)
    resp = session.get(url, timeout=timeout)
    resp.raise_for_status()
    return resp.text

def download_image_bytes(url: str, referer: Optional[str] = None, timeout: int = 25) -> Tuple[Optional[bytes], Optional[str]]:
    """Tải nội dung ảnh dạng bytes và trả về (bytes, content_type)"""
    # Xử lý Base64 Data URI trực tiếp
    if url.startswith("data:image/"):
        from .parsers.image_decoder import ImageDecoder
        res = ImageDecoder.decode_base64_image(url)
        if res:
            return res[0], "image/jpeg"

    from .network import default_pipeline
    return default_pipeline.fetch_image_smart(url, referer=referer, timeout=timeout)

def get_session(referer: Optional[str] = None) -> requests.Session:
    """Tạo requests session với headers phù hợp"""
    session = requests.Session()
    headers = DEFAULT_HEADERS.copy()
    if referer:
        headers["Referer"] = referer
    session.headers.update(headers)
    return session
