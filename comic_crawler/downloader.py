import os
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import List, Optional, Tuple, Dict
from rich.progress import Progress

from .models import Manga, Chapter
from .utils import (
    clean_filename,
    format_chapter_name,
    format_page_filename,
    download_image_bytes,
    resolve_output_dir
)
from .packager import create_cbz_from_memory, optimize_and_enhance_image

class ComicDownloader:
    def __init__(
        self,
        output_dir: Optional[str] = None,
        max_workers: int = 5,
        enhance_images: bool = True
    ):
        self.output_dir = resolve_output_dir(output_dir)
        self.max_workers = max_workers
        self.enhance_images = enhance_images
        self.output_dir.mkdir(parents=True, exist_ok=True)

    def _fetch_and_process_page(
        self,
        page_idx: int,
        img_url: str,
        referer: Optional[str] = None,
        retries: int = 3
    ) -> Optional[Tuple[int, str, bytes]]:
        """Tải ảnh vào RAM và xử lý làm đẹp/nét ảnh cho máy đọc sách"""
        for attempt in range(retries):
            content, _ = download_image_bytes(img_url, referer=referer)
            if content:
                # Xử lý làm nét & tối ưu ảnh cho máy đọc sách trong RAM
                processed_bytes, ext = optimize_and_enhance_image(
                    raw_bytes=content,
                    enhance=self.enhance_images
                )
                filename = format_page_filename(page_idx, ext)
                return page_idx, filename, processed_bytes

            time.sleep(1.0 * (attempt + 1))

        return None

    def download_chapter(
        self,
        manga: Manga,
        chapter: Chapter,
        extractor_referer: Optional[str] = None,
        progress: Optional[Progress] = None,
        task_id = None
    ) -> Optional[str]:
        """
        Tải toàn bộ các trang của chương vào RAM, làm đẹp và đóng gói thẳng thành file .cbz.
        Hoàn toàn không lưu folder ảnh tạm xuống ổ cứng!
        """
        safe_manga_title = clean_filename(manga.title)
        safe_chap_title = format_chapter_name(chapter.number, chapter.title)

        # Thư mục truyện
        manga_dir = self.output_dir / safe_manga_title
        manga_dir.mkdir(parents=True, exist_ok=True)

        # File CBZ đích
        cbz_path = manga_dir / f"{safe_chap_title}.cbz"

        pages = chapter.pages
        total_pages = len(pages)
        if total_pages == 0:
            return None

        # Tải & xử lý ảnh song song hoàn toàn trong bộ nhớ RAM
        pages_dict: Dict[int, Tuple[str, bytes]] = {}
        download_tasks = []

        with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
            for idx, img_url in enumerate(pages, start=1):
                task = executor.submit(
                    self._fetch_and_process_page,
                    page_idx=idx,
                    img_url=img_url,
                    referer=extractor_referer or chapter.url
                )
                download_tasks.append(task)

            for task in as_completed(download_tasks):
                res = task.result()
                if res:
                    page_idx, filename, img_bytes = res
                    pages_dict[page_idx] = (filename, img_bytes)

                if progress and task_id:
                    progress.advance(task_id, 1)

        if not pages_dict:
            return None

        # Sắp xếp các trang đúng thứ tự 1, 2, 3...
        sorted_pages = [pages_dict[i] for i in sorted(pages_dict.keys())]

        # Đóng gói trực tiếp thành file .cbz
        create_cbz_from_memory(
            output_cbz_path=str(cbz_path),
            pages_data=sorted_pages,
            manga_title=manga.title,
            chapter_number=chapter.number,
            chapter_title=chapter.title
        )

        return str(cbz_path)
