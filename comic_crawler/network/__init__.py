from .base_fetcher import BaseFetcher
from .tls_fetcher import TlsImpersonationFetcher
from .mirror_resolver import MirrorResolver
from .headers_provider import HeadersProvider
from .pipeline import NetworkPipeline

# Singleton pipeline dùng chung trong toàn bộ ứng dụng
default_pipeline = NetworkPipeline()

__all__ = [
    "BaseFetcher",
    "TlsImpersonationFetcher",
    "MirrorResolver",
    "HeadersProvider",
    "NetworkPipeline",
    "default_pipeline",
]

