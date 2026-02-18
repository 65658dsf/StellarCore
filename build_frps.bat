@echo off
:: 设置控制台编码为UTF-8
chcp 65001 >nul 2>&1
setlocal EnableDelayedExpansion

:: 版本号
set VERSION=1.1.5
set BINARY_NAME=StellarCore


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

:: Windows
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

:: Linux
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

:: FreeBSD
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

:: macOS
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

echo.
echo 编译完成！
echo 输出文件在 output 目录中

endlocal
