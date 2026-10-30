from typing import Optional, List, Dict
from urllib.parse import urlparse, urlunparse

class MirrorResolver:
    """
    Xử lý DẠNG 1 (Mở rộng): Cloudflare Domain Lock & Mirror Failover.
    Các hệ thống web truyện lớn (NetTruyen, TruyenQQ, MangaKakalot...) luôn dùng
    hàng chục domain mirror. Khi một domain bị Cloudflare bật 'Just a moment...',
    Resolver tự động chuyển sang mirror tương ứng không bị khóa.
    """

    KNOWN_CLUSTERS: Dict[str, List[str]] = {
        "nettruyen": [
            "nettruyen.gg",
            "nettruyen.cx",
            "nettruyenco.vn",
            "nettruyena.com",
            "nettruyenbb.com",
            "nettruyenviet1.com",
            "nettruyenmax.com"
        ],
        "truyenqq": [
            "truyenqqviet.com",
            "truyenqq.com.co",
            "truyenqqto.com",
            "truyenqqvip.com"
        ],
        "mangakakalot": [
            "mangakakalot.com",
            "manganato.com",
            "chapmanganato.to",
            "readmanganato.com"
        ]
    }

    @classmethod
    def get_candidate_mirrors(cls, original_url: str) -> List[str]:
        """Trả về danh sách các URL mirror tiềm năng cho 1 URL bị chặn"""
        parsed = urlparse(original_url)
        netloc = parsed.netloc.lower()

        for cluster_key, mirror_domains in cls.KNOWN_CLUSTERS.items():
            if cluster_key in netloc:
                candidates = []
                for domain in mirror_domains:
                    if domain not in netloc:
                        new_url = urlunparse((
                            parsed.scheme or "https",
                            domain,
                            parsed.path,
                            parsed.params,
                            parsed.query,
                            parsed.fragment
                        ))
                        candidates.append(new_url)
                return candidates

        return []

