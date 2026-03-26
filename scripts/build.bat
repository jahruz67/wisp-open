@echo off
setlocal enabledelayedexpansion
pushd "%~dp0\.."

REM Build script for wis-free-v3 Voice Dictation App
REM Builds a production-ready Windows executable with icon

echo ========================================
echo    wis-free-v3 - Build Script
echo ========================================
echo.

REM Check if wails is installed
where wails >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Wails CLI not found. Please install it first:
    echo         go install github.com/wailsapp/wails/v2/cmd/wails@latest
    pause
    exit /b 1
)

echo [1/2] Cleaning old build...
if exist build\bin\wis-free-v3.exe del build\bin\wis-free-v3.exe

echo [2/2] Building with Wails...
echo       (This includes the app icon and frontend)
echo.

REM Ensure CGO is enabled for native dependencies
set CGO_ENABLED=1

REM Check if GCC is in the PATH
where gcc >nul 2>&1
if %ERRORLEVEL% EQU 0 goto :gcc_check_done
echo [WARNING] GCC compiler not found in PATH.
echo           A compiler is required for native Windows features.
echo           You can download MinGW-w64 from: https://winlibs.com/
echo.
set /p MINGW_PATH="Paste your MinGW/bin folder path here (or press Enter to skip): "
echo.
if "!MINGW_PATH!"=="" goto :gcc_check_done

REM Remove quotes if the user provided them
set MINGW_PATH=!MINGW_PATH:"=!

REM Check if the path exists
if not exist "!MINGW_PATH!" (
    echo [ERROR] Path "!MINGW_PATH!" does not exist.
    goto :gcc_check_done
)

REM Look for gcc.exe in 3 places: provided path, path\bin, and path\..\bin
if not exist "!MINGW_PATH!\gcc.exe" (
    if exist "!MINGW_PATH!\bin\gcc.exe" (
        set "MINGW_PATH=!MINGW_PATH!\bin"
    ) else if exist "!MINGW_PATH!\..\bin\gcc.exe" (
        set "MINGW_PATH=!MINGW_PATH!\..\bin"
    )
)

set "PATH=!MINGW_PATH!;%PATH%"
echo [INFO] Detected GCC at: "!MINGW_PATH!"

:gcc_check_done

REM Final check for GCC
where gcc >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] GCC is still not found. Native compilation will fail.
    echo         Please install MinGW-w64 e.g., from https://winlibs.com/
    pause
    exit /b 1
)

wails build -clean -ldflags="-linkmode internal" -skipbindings

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Build failed!
    pause
    exit /b 1
)

echo.
echo ========================================
echo    BUILD SUCCESSFUL!
echo ========================================
echo.
echo    Output: build\bin\wis-free-v3.exe
echo.
echo    NOTE: Close any running instance before
echo          replacing wis-free-v3.exe in this folder.
echo.
dir build\bin\wis-free-v3.exe | find "wis-free-v3.exe"
echo.

pause
