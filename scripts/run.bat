@echo off
setlocal enabledelayedexpansion
pushd "%~dp0\.."

echo Setting up wis-free-v3...

REM Create config directory
if not exist "%USERPROFILE%\.wis-free-v3" mkdir "%USERPROFILE%\.wis-free-v3"

REM Copy config if it exists
if exist "config.json" (
    copy /Y "config.json" "%USERPROFILE%\.wis-free-v3\config.json" >nul
    echo Config copied successfully.
)

echo Starting wis-free-v3...
echo.
echo Press 'k' to start/stop recording.
echo Check this console for transcription output.
echo.

REM Add Go and GCC to PATH and run
set PATH=%PATH%;C:\Program Files\Go\bin;C:\TDM-GCC-64\bin

if exist "build\bin\wis-free-v3.exe" (
    build\bin\wis-free-v3.exe
) else if exist "wis-free-v3.exe" (
    wis-free-v3.exe
) else (
    echo [ERROR] wis-free-v3.exe not found. Please run scripts/build.bat first.
)

popd
pause
