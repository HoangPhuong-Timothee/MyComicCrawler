@echo off
chcp 65001 > nul
title Comic Crawler Engine (Golang High-Speed & Loop)
echo =======================================================
echo    COMIC CRAWLER ENGINE FOR ANDROID E-READER
echo =======================================================
echo.

if exist "comic-crawler.exe" (
    comic-crawler.exe
) else (
    echo [INFO] Running Python fallback engine...
    python main.py
)

echo.
pause
