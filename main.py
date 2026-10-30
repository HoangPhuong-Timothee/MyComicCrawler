import os
import sys
import argparse
from typing import List
from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich.prompt import Prompt, Confirm
from rich.progress import Progress, SpinnerColumn, BarColumn, TextColumn, TimeRemainingColumn

from comic_crawler.extractors import get_extractor
from comic_crawler.downloader import ComicDownloader
from comic_crawler.models import Manga, Chapter
from comic_crawler.utils import resolve_output_dir, clean_filename

console = Console()
DEFAULT_OUTPUT_DIR = r"C:\Users\ROSA\workspace\comic"

def parse_chapter_selection(selection_str: str, chapters: List[Chapter]) -> List[Chapter]:
    """
    Phân tích chuỗi lựa chọn chapter:
    - 'all': Tất cả các chapter
    - '1-10': Từ chapter 1 đến 10 theo số thứ tự chapter (number) hoặc index
    - '5,8,12': Các chapter cụ thể
    - '50': Chương cụ thể
    """
    selection = selection_str.strip().lower()
    if not selection or selection == "all":
        return chapters

    selected = []
    tokens = [t.strip() for t in selection.split(',') if t.strip()]

    for token in tokens:
        if '-' in token:
            parts = token.split('-')
            try:
                start = float(parts[0].strip())
                end = float(parts[1].strip())
                matched = [ch for ch in chapters if start <= ch.number <= end]
                selected.extend(matched)
            except ValueError:
                pass
        else:
            try:
                num = float(token)
                matched = [ch for ch in chapters if abs(ch.number - num) < 0.001]
                if matched:
                    selected.extend(matched)
                else:
                    # Nếu không tìm thấy theo number, thử theo index (1-based)
                    idx = int(num) - 1
                    if 0 <= idx < len(chapters):
                        selected.append(chapters[idx])
            except ValueError:
                pass

    seen = set()
    unique_selected = []
    for ch in selected:
        if ch.url not in seen:
            seen.add(ch.url)
            unique_selected.append(ch)

    return unique_selected if unique_selected else chapters

def print_banner():
    banner_text = """[bold cyan]📖 COMIC TO CBZ CRAWLER FOR ANDROID E-READER 📖[/bold cyan]
[dim]Tải ảnh vào RAM -> Tự động làm nét -> Đóng gói thẳng file .cbz (Không tạo folder ảnh rác)[/dim]"""
    console.print(Panel(banner_text, border_style="cyan"))

def main():
    parser = argparse.ArgumentParser(description="Tool tự động cào truyện tranh đóng gói trực tiếp thành file .cbz cho máy đọc sách Android.")
    parser.add_argument("url", nargs="?", help="URL truyện hoặc chapter cần cào")
    parser.add_argument("-o", "--output", default=DEFAULT_OUTPUT_DIR, help=f"Thư mục lưu truyện (mặc định: {DEFAULT_OUTPUT_DIR})")
    parser.add_argument("-c", "--chapters", default=None, help="Lựa chọn chương để tải (vd: 'all', '1-10', '5,8,12')")
    parser.add_argument("--no-enhance", action="store_true", help="Không tự động làm nét nét vẽ & tăng tương phản cho máy đọc sách")
    parser.add_argument("-w", "--workers", type=int, default=5, help="Số luồng tải ảnh đồng thời (mặc định: 5)")

    args = parser.parse_args()

    print_banner()

    # Chế độ Interactive nếu không truyền URL qua CLI
    if not args.url:
        target_url = Prompt.ask("[bold yellow]👉 Nhập URL truyện (hoặc chương)[/bold yellow]").strip()
        if not target_url:
            console.print("[red]❌ URL không được để trống![/red]")
            return
        
        raw_output_dir = Prompt.ask(
            "[bold yellow]📂 Thư mục lưu truyện (nhấn Enter để dùng mặc định, hoặc nhập tên thư mục con)[/bold yellow]",
            default=DEFAULT_OUTPUT_DIR
        ).strip()
        
        enhance_images = Confirm.ask(
            "[bold yellow]✨ Tối ưu & tăng độ nét chữ/nét vẽ cho máy đọc sách (E-ink/Android)?[/bold yellow]",
            default=True
        )
    else:
        target_url = args.url
        raw_output_dir = args.output
        enhance_images = not args.no_enhance

    # Chuẩn hóa thư mục lưu để không bao giờ bị lưu vào source code
    output_dir = resolve_output_dir(raw_output_dir)
    console.print(f"[bold cyan]📁 Thư mục lưu đích:[/bold cyan] [bold yellow]{output_dir}[/bold yellow]")

    # Xác định Extractor
    extractor = get_extractor(target_url)
    console.print(f"[green]✓[/green] Đang sử dụng module phân tích: [bold magenta]{extractor.name}[/bold magenta]")

    # Phân tích thông tin truyện
    with console.status(f"[cyan]Đang quét thông tin truyện từ: {target_url}...[/cyan]"):
        try:
            manga = extractor.get_manga(target_url)
        except Exception as e:
            console.print(f"[red]❌ Lỗi khi lấy thông tin truyện:[/red] {e}")
            return

    console.print(f"\n[bold green]📚 Tên truyện:[/bold green] [bold white]{manga.title}[/bold white]")
    console.print(f"[bold green]📑 Tổng số chương tìm thấy:[/bold green] {len(manga.chapters)}")

    if not manga.chapters:
        console.print("[red]❌ Không tìm thấy chương nào từ URL này. Hãy kiểm tra lại link hoặc cấu trúc trang web.[/red]")
        return

    # Lựa chọn chapters cần tải
    if not args.chapters:
        # Hiển thị vài chapter mẫu
        preview_table = Table(title="Danh sách chương (Mẫu)", show_lines=False)
        preview_table.add_column("STT", style="cyan", width=6)
        preview_table.add_column("Tên chương", style="white")
        preview_table.add_column("Số chap", style="green", width=10)

        preview_count = min(5, len(manga.chapters))
        for i in range(preview_count):
            ch = manga.chapters[i]
            preview_table.add_row(str(i + 1), ch.title, str(ch.number))
        
        if len(manga.chapters) > 10:
            preview_table.add_row("...", "...", "...")
            for i in range(len(manga.chapters) - 2, len(manga.chapters)):
                ch = manga.chapters[i]
                preview_table.add_row(str(i + 1), ch.title, str(ch.number))

        console.print(preview_table)

        chap_choice = Prompt.ask(
            "\n[bold yellow]👉 Chọn chương cần tải ('all' để tải hết, hoặc '1-5', '10,12', hoặc số chap)[/bold yellow]",
            default="all"
        )
    else:
        chap_choice = args.chapters

    selected_chapters = parse_chapter_selection(chap_choice, manga.chapters)
    console.print(f"[cyan]🚀 Số chương đã chọn để tải:[/cyan] [bold green]{len(selected_chapters)}[/bold green]")

    downloader = ComicDownloader(
        output_dir=output_dir,
        max_workers=args.workers,
        enhance_images=enhance_images
    )

    # Bắt đầu tải các chương
    safe_manga_name = clean_filename(manga.title)
    final_manga_folder = output_dir / safe_manga_name
    console.print(f"\n[bold cyan]⬇ Đang tải & nén trực tiếp vào:[/bold cyan] {final_manga_folder}")
    total_chapters = len(selected_chapters)

    for idx, chapter in enumerate(selected_chapters, start=1):
        console.print(f"\n[bold blue][{idx}/{total_chapters}][/bold blue] Đang xử lý: [bold yellow]{chapter.title}[/bold yellow] (Chap {chapter.number})")

        # Nếu chapter chưa có danh sách link ảnh, gọi extractor để lấy
        if not chapter.pages:
            with console.status(" [cyan]Đang lấy danh sách trang ảnh...[/cyan]"):
                try:
                    chapter.pages = extractor.get_chapter_images(chapter.url)
                except Exception as e:
                    console.print(f" [red]⚠ Không lấy được ảnh của chương {chapter.title}: {e}[/red]")
                    continue

        if not chapter.pages:
            console.print(f" [yellow]⚠ Chương này không có ảnh nào để tải.[/yellow]")
            continue

        console.print(f" [green]✓ Tìm thấy {len(chapter.pages)} trang. Tải vào RAM, làm nét & đóng gói .cbz...[/green]")

        # Thanh tiến trình tải & nén ảnh
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
            TextColumn("({task.completed}/{task.total} trang)"),
            TimeRemainingColumn(),
            console=console
        ) as progress:
            task = progress.add_task("  Đang xử lý trang...", total=len(chapter.pages))
            cbz_dest = downloader.download_chapter(
                manga=manga,
                chapter=chapter,
                extractor_referer=extractor.get_referer(chapter.url),
                progress=progress,
                task_id=task
            )

        if cbz_dest:
            console.print(f"  [bold green]✓ Tạo thành công file .cbz:[/bold green] [dim]{cbz_dest}[/dim]")
        else:
            console.print(f"  [red]❌ Không thể tải trang nào của chương này.[/red]")

    # Thông báo hoàn thành và hướng dẫn chép sang máy đọc sách
    success_text = f"""[bold green]🎉 TẤT CẢ CÁC CHƯƠNG ĐÃ ĐƯỢC TẠI THÀNH FILE .CBZ![/bold green]

[bold yellow]📂 Thư mục truyện trên máy tính (chỉ chứa các file .cbz siêu gọn nhẹ):[/bold yellow]
{final_manga_folder}

[bold cyan]📱 HƯỚNG DẪN CHÉP VÀO MÁY ĐỌC SÁCH ANDROID:[/bold cyan]
1. Cắm cáp USB máy đọc sách vào máy tính (chọn chế độ File Transfer / MTP).
2. Chép thư mục [bold]"{safe_manga_name}"[/bold] vào máy đọc sách:
   • [bold green]Mihon / Tachiyomi / Kotatsu:[/bold green]
     Chép vào thư mục: [dim]Mihon/local/{safe_manga_name}[/dim] hoặc [dim]Tachiyomi/local/{safe_manga_name}[/dim]
     Sau đó vào app -> mục [b]Duyệt[/b] -> chọn tab [b]Nguồn cục bộ[/b] là truyện hiện ra đầy đủ!
   • [bold green]Moon+ Reader / Perfect Viewer / Kuro Reader / Boox NeoReader:[/bold green]
     Chép vào thư mục [dim]Books[/dim] hoặc [dim]Comics[/dim] rồi mở app đọc trực tiếp!
"""
    console.print(Panel(success_text, border_style="green"))

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        console.print("\n[yellow]Đã hủy tác vụ bởi người dùng.[/yellow]")
        sys.exit(0)
