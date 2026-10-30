import io
import zipfile
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import List, Tuple, Optional
from PIL import Image, ImageEnhance, ImageOps

from .utils import detect_image_extension

def generate_comic_info(manga_title: str, chapter_number: float, chapter_title: str = "") -> str:
    """Tạo ComicInfo.xml chuẩn theo ComicRack/Mihon/Tachiyomi metadata schema"""
    root = ET.Element("ComicInfo")
    
    series_elem = ET.SubElement(root, "Series")
    series_elem.text = manga_title
    
    if chapter_title:
        title_elem = ET.SubElement(root, "Title")
        title_elem.text = chapter_title
        
    num_elem = ET.SubElement(root, "Number")
    num_elem.text = str(chapter_number) if chapter_number != 0 else "1"

    return ET.tostring(root, encoding="utf-8", xml_declaration=True).decode("utf-8")

def optimize_and_enhance_image(raw_bytes: bytes, enhance: bool = True) -> Tuple[bytes, str]:
    """
    Xử lý ảnh trực tiếp trong RAM (In-Memory):
    - Chuyển đổi định dạng sang JPG chất lượng cao (tương thích 100% mọi máy đọc sách).
    - Khử nền trong suốt RGBA sang nền trắng (tránh bị đen nền).
    - Tăng độ nét nét vẽ manga và chữ thoại (Sharpness).
    - Tăng nhẹ độ tương phản (Contrast) giúp màn hình E-ink hiển thị rõ ràng, đậm đà hơn.
    """
    try:
        with Image.open(io.BytesIO(raw_bytes)) as img:
            # 1. Xử lý kênh màu: chuyển về RGB chuẩn
            if img.mode in ("RGBA", "LA") or (img.mode == "P" and "transparency" in img.info):
                # Nền trắng cho ảnh có kênh alpha
                background = Image.new("RGB", img.size, (255, 255, 255))
                rgba_img = img.convert("RGBA")
                background.paste(rgba_img, mask=rgba_img.split()[3])
                processed = background
            elif img.mode != "RGB":
                processed = img.convert("RGB")
            else:
                processed = img.copy()

            # 2. Tinh chỉnh làm nét & tăng tương phản cho máy đọc sách
            if enhance:
                # Tăng độ nét cho nét vẽ line-art & chữ thoại
                sharpness_enhancer = ImageEnhance.Sharpness(processed)
                processed = sharpness_enhancer.enhance(1.25)

                # Tăng nhẹ tương phản để chữ đen đậm hơn trên nền trắng e-ink
                contrast_enhancer = ImageEnhance.Contrast(processed)
                processed = contrast_enhancer.enhance(1.08)

            # 3. Xuất ra buffer bytes JPG chất lượng cao
            output_buffer = io.BytesIO()
            processed.save(output_buffer, format="JPEG", quality=92, optimize=True)
            return output_buffer.getvalue(), ".jpg"

    except Exception:
        # Nếu có lỗi khi giải mã ảnh (file hỏng nhẹ hoặc định dạng lạ), giữ nguyên bytes ban đầu
        ext = detect_image_extension(None, "", raw_bytes[:32])
        return raw_bytes, ext

def create_cbz_from_memory(
    output_cbz_path: str,
    pages_data: List[Tuple[str, bytes]],
    manga_title: str = "",
    chapter_number: float = 0,
    chapter_title: str = ""
) -> str:
    """
    Đóng gói trực tiếp danh sách ảnh từ bộ nhớ RAM thành file .cbz:
    - Không cần lưu bất kỳ folder ảnh hay file tạm nào xuống ổ cứng.
    - Dùng ZIP_STORED để máy đọc sách mở trang tức thì (0ms latency, tiết kiệm pin).
    """
    cbz_path = Path(output_cbz_path)
    cbz_path.parent.mkdir(parents=True, exist_ok=True)

    with zipfile.ZipFile(cbz_path, "w", compression=zipfile.ZIP_STORED) as zip_file:
        # 1. Ghi ComicInfo.xml metadata
        if manga_title:
            comic_info_xml = generate_comic_info(manga_title, chapter_number, chapter_title)
            zip_file.writestr("ComicInfo.xml", comic_info_xml)

        # 2. Ghi từng trang ảnh trực tiếp từ RAM
        for filename, img_bytes in pages_data:
            zip_file.writestr(filename, img_bytes)

    return str(cbz_path)
