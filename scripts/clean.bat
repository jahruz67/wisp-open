@echo off
setlocal enabledelayedexpansion
pushd "%~dp0\.."

echo ========================================
echo    wis-free-v3 - Clean Script
echo ========================================
echo.

REM --- Build output ---
echo [1/6] Removing build binaries...
if exist build\bin\wis-free-v3.exe del /f /q build\bin\wis-free-v3.exe 2>nul
if exist build\bin\wis-free-v3.tar.gz del /f /q build\bin\wis-free-v3.tar.gz 2>nul
if exist tmp.exe del /f /q tmp.exe 2>nul

echo [2/6] Removing Wails build cache...
if exist "%USERPROFILE%\AppData\Local\wails" (
    rmdir /s /q "%USERPROFILE%\AppData\Local\wails" 2>nul
)

echo [3/6] Removing frontend dist and node_modules...
if exist frontend\dist rmdir /s /q frontend\dist 2>nul
if exist frontend\node_modules rmdir /s /q frontend\node_modules 2>nul
if exist frontend\package-lock.json del /f /q frontend\package-lock.json 2>nul
if exist frontend\package.json.md5 del /f /q frontend\package.json.md5 2>nul

echo [4/6] Removing Wails generated JS bindings...
if exist frontend\wailsjs rmdir /s /q frontend\wailsjs 2>nul

echo [5/6] Removing Go build cache (optional - comment out if you want to keep it)...
REM go clean -cache -testcache -modcache

echo [6/6] Removing any leftover .exe and .log files in root...
if exist *.exe del /f /q *.exe 2>nul
if exist *.log del /f /q *.log 2>nul

echo.
echo ========================================
echo    CLEAN COMPLETE
echo ========================================
echo.
echo Cleaned: build binaries, frontend dist/node_modules,
echo          wailsjs bindings, Wails cache, temp files.
echo.
echo To rebuild, run: scripts\build.bat
echo.

popd