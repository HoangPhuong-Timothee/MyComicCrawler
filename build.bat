@echo off
chcp 65001 > nul
title Build Comic Crawler Binary
echo ========================================================
echo       BUILDING COMIC CRAWLER (comic-crawler.exe)
echo ========================================================
echo.

where go >nul 2>nul
if %errorlevel% neq 0 set "PATH=%PATH%;C:\Program Files\Go\bin"

where go >nul 2>nul
if %errorlevel% neq 0 goto :no_go

echo [1/2] Closing running processes if any...
taskkill /F /IM comic-crawler.exe >nul 2>nul

echo [2/2] Compiling Go code...
go build -ldflags="-s -w" -o comic-crawler.exe ./cmd/comic-crawler

if %errorlevel% equ 0 goto :success
goto :failed

:no_go
echo [ERROR] Golang not found on your system!
echo Please install Go from https://go.dev/dl/ or add Go to PATH.
goto :end

:success
echo.
echo ========================================================
echo   BUILD SUCCESS: comic-crawler.exe
echo ========================================================
echo You can run it now by double-clicking run.bat
goto :end

:failed
echo.
echo [ERROR] Build failed! Please check your Go code syntax.
goto :end

:end
echo.
pause
