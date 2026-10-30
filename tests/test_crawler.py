import os
import sys
import shutil
import zipfile
from pathlib import Path

# Thêm đường dẫn gốc dự án vào sys.path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from comic_crawler.models import Manga, Chapter
from comic_crawler.packager import create_cbz_from_memory, optimize_and_enhance_image, generate_comic_info
from comic_crawler.utils import (
    format_chapter_name,
    format_page_filename,
    clean_filename,
    resolve_output_dir,
    DEFAULT_COMIC_DIR
)
from PIL import Image
import io

def test_utils():
    assert clean_filename("One Piece: Tập 1 / Wano ? *") == "One Piece_ Tập 1 _ Wano _ _"
    assert format_chapter_name(1, "Khởi đầu") == "Chapter 001 - Khởi đầu"
    assert format_chapter_name(12.5, "Extra") == "Chapter 012.5 - Extra"
    assert format_page_filename(1, ".jpg") == "001.jpg"
    assert format_page_filename(25, ".png") == "025.png"
    
    # Test resolve_output_dir:
    assert resolve_output_dir(None) == DEFAULT_COMIC_DIR
    assert resolve_output_dir("") == DEFAULT_COMIC_DIR
    assert resolve_output_dir("20centuryboys") == DEFAULT_COMIC_DIR / "20centuryboys"
    assert "comic-craw-tool" not in str(resolve_output_dir("20centuryboys"))
    # Test passing a Path object directly:
    p = DEFAULT_COMIC_DIR / "20centuryboys"
    assert resolve_output_dir(p) == p.resolve()
    
    print("Utils test PASSED!")

def test_in_memory_cbz_and_enhancement():
    # 1. Tạo 1 ảnh mẫu bằng Pillow trong RAM
    im = Image.new("RGBA", (200, 300), color=(255, 0, 0, 200))
    buf = io.BytesIO()
    im.save(buf, format="PNG")
    raw_bytes = buf.getvalue()

    # 2. Test hàm optimize_and_enhance_image
    proc_bytes, ext = optimize_and_enhance_image(raw_bytes, enhance=True)
    assert ext == ".jpg"
    assert len(proc_bytes) > 0
    # Đảm bảo ảnh mở lại được và là RGB
    with Image.open(io.BytesIO(proc_bytes)) as test_img:
        assert test_img.mode == "RGB"
        assert test_img.size == (200, 300)

    # 3. Test hàm create_cbz_from_memory
    test_cbz_path = Path("test_output/test_chap.cbz")
    pages_data = [
        ("001.jpg", proc_bytes),
        ("002.jpg", proc_bytes)
    ]
    cbz_res = create_cbz_from_memory(
        output_cbz_path=str(test_cbz_path),
        pages_data=pages_data,
        manga_title="20th Century Boys",
        chapter_number=1,
        chapter_title="Khởi đầu"
    )
    assert test_cbz_path.exists()

    # 4. Kiểm tra nội dung bên trong file CBZ
    with zipfile.ZipFile(test_cbz_path, "r") as z:
        namelist = z.namelist()
        assert "001.jpg" in namelist
        assert "002.jpg" in namelist
        assert "ComicInfo.xml" in namelist
        xml_content = z.read("ComicInfo.xml").decode("utf-8")
        assert "<Series>20th Century Boys</Series>" in xml_content
        assert "<Number>1</Number>" in xml_content

    # Dọn dẹp
    shutil.rmtree("test_output", ignore_errors=True)
    print("In-Memory CBZ & Enhancement test PASSED!")

if __name__ == "__main__":
    test_utils()
    test_in_memory_cbz_and_enhancement()
    print("ALL TESTS PASSED SUCCESSFULLY!")
