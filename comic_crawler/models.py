from dataclasses import dataclass, field
from typing import List, Optional

@dataclass
class Chapter:
    id: str
    title: str
    url: str
    number: float = 0.0
    pages: List[str] = field(default_factory=list)

@dataclass
class Manga:
    title: str
    url: str
    cover_url: Optional[str] = None
    author: Optional[str] = None
    description: Optional[str] = None
    chapters: List[Chapter] = field(default_factory=list)

