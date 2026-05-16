@echo off
:: 设置控制台编码为UTF-8
chcp 65001 >nul 2>&1
setlocal EnableDelayedExpansion

:: 版本号
set VERSION=1.1.7
set BINARY_NAME=StellarCore

set BUILD_WINDOWS=0
set BUILD_LINUX=0
set BUILD_FREEBSD=0
set BUILD_MACOS=0

if "%~1"=="" goto build_all_targets
goto parse_targets

:build_all_targets
set BUILD_WINDOWS=1
set BUILD_LINUX=1
set BUILD_FREEBSD=1
set BUILD_MACOS=1
goto targets_done

:parse_targets
if "%~1"=="" goto targets_done
set TARGET=%~1
if "%TARGET:~0,2%"=="--" set TARGET=%TARGET:~2%
if "%TARGET:~0,1%"=="-" set TARGET=%TARGET:~1%
if "%TARGET:~0,1%"=="/" set TARGET=%TARGET:~1%
set TARGET_MATCHED=0
if /I "%TARGET%"=="all" (
    set BUILD_WINDOWS=1
    set BUILD_LINUX=1
    set BUILD_FREEBSD=1
    set BUILD_MACOS=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="windows" (
    set BUILD_WINDOWS=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="linux" (
    set BUILD_LINUX=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="freebsd" (
    set BUILD_FREEBSD=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="freedbs" (
    set BUILD_FREEBSD=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="macos" (
    set BUILD_MACOS=1
    set TARGET_MATCHED=1
)
if /I "%TARGET%"=="darwin" (
    set BUILD_MACOS=1
    set TARGET_MATCHED=1
)
if "%TARGET_MATCHED%"=="0" (
    echo 错误：未知构建目标 "%TARGET%"
    echo 用法：build_frps.bat [all] [windows] [linux] [freebsd] [macos]
    exit /b 1
)
shift
goto parse_targets

:targets_done
echo 构建目标: windows=%BUILD_WINDOWS% linux=%BUILD_LINUX% freebsd=%BUILD_FREEBSD% macos=%BUILD_MACOS%

:: 检查是否安装了 Node.js/npm
where npm >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 错误：请先安装 Node.js 和 npm！
    echo 下载地址：https://nodejs.org/
    exit /b 1
)

:: 清空输出目录
if exist output (
    echo 正在清空 output\StellarCore 目录...
    rmdir /s /q output\StellarCore
)
mkdir output
mkdir output\StellarCore

:: 检查是否安装了 7-Zip
where 7z >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 错误：请先安装 7-Zip！
    echo 下载地址：https://7-zip.org/
    exit /b 1
)

:: 检查是否安装了 UPX
where upx >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 错误：请先安装 UPX！
    echo 下载地址：https://github.com/upx/upx/releases
    exit /b 1
)

:: 编译并同步 frps 前端资源
echo 正在编译 frps 管理面板前端...
pushd web\frps
call npm run build-only
if %ERRORLEVEL% NEQ 0 (
    popd
    echo 错误：frps 前端编译失败！
    exit /b 1
)
popd

echo 正在同步 frps 前端资源到 assets\frps\static...
if not exist web\frps\dist (
    echo 错误：web\frps\dist 不存在，无法同步前端资源！
    exit /b 1
)
if exist assets\frps\static (
    rmdir /s /q assets\frps\static
)
mkdir assets\frps\static
xcopy "web\frps\dist\*" "assets\frps\static\" /E /I /H /Y >nul
if %ERRORLEVEL% GEQ 2 (
    echo 错误：同步 frps 前端资源失败！
    exit /b 1
)

:: Windows
if not "%BUILD_WINDOWS%"=="1" goto after_windows
echo 正在编译 Windows 版本...
set GOOS=windows
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%BINARY_NAME%_%VERSION%_windows_386.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%BINARY_NAME%_%VERSION%_windows_amd64.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%BINARY_NAME%_%VERSION%_windows_arm32.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%BINARY_NAME%_%VERSION%_windows_arm64.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"
:after_windows

:: Linux
if not "%BUILD_LINUX%"=="1" goto after_linux
echo 正在编译 Linux 版本...
set GOOS=linux
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_386.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_386.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_386.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_386.tar"

set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_amd64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_amd64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_amd64.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=5
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v5.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v5.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v5.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_arm32v5.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=6
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v6.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v6.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v6.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_arm32v6.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v7.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v7.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm32v7.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_arm32v7.tar"

set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_linux_arm64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_linux_arm64.tar"
:after_linux

:: FreeBSD
if not "%BUILD_FREEBSD%"=="1" goto after_freebsd
echo 正在编译 FreeBSD 版本...
set GOOS=freebsd
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_386.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_386.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_386.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_freebsd_386.tar"

set GOOS=freebsd
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_amd64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_amd64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_freebsd_amd64.tar"

set GOOS=freebsd
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm32.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm32.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm32.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_freebsd_arm32.tar"

set GOOS=freebsd
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_freebsd_arm64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_freebsd_arm64.tar"
:after_freebsd

:: macOS
if not "%BUILD_MACOS%"=="1" goto after_macos
echo 正在编译 macOS 版本...
set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_amd64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_amd64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_darwin_amd64.tar"

set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frps
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output/StellarCore && 7z a -ttar "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_arm64.tar.gz" "../StellarCore/%BINARY_NAME%_%VERSION%_darwin_arm64.tar" && cd .. && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%BINARY_NAME%_%VERSION%_darwin_arm64.tar"
:after_macos

echo.
echo 编译完成！
echo 输出文件在 output 目录中

endlocal
