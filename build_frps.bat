@echo off
:: 设置控制台编码为UTF-8
chcp 65001 >nul 2>&1
setlocal EnableDelayedExpansion

:: 版本号
set VERSION=1.1.0
set BINARY_NAME=StellarCore

:: 清空输出目录
if exist output (
    echo 正在清空 output 目录...
    rmdir /s /q output
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
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%VERSION%_windows_386_%GIT_VERSION%.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%VERSION%_windows_amd64_%GIT_VERSION%.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%VERSION%_windows_arm32_%GIT_VERSION%.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%.exe" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%.exe"
powershell -Command "Compress-Archive -Path 'output/StellarCore/%BINARY_NAME%.exe' -DestinationPath 'output/StellarCore/%VERSION%_windows_arm64_%GIT_VERSION%.zip' -Force"
del "output\StellarCore\%BINARY_NAME%.exe"

:: Linux
echo 正在编译 Linux 版本...
set GOOS=linux
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_386_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_386_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_386_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_386_%GIT_VERSION%.tar"

set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_amd64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_amd64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_amd64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_amd64_%GIT_VERSION%.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=5
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_arm32v5_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_arm32v5_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_arm32v5_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_arm32v5_%GIT_VERSION%.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=6
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_arm32v6_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_arm32v6_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_arm32v6_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_arm32v6_%GIT_VERSION%.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_arm32v7_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_arm32v7_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_arm32v7_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_arm32v7_%GIT_VERSION%.tar"

set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_linux_arm64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_linux_arm64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_linux_arm64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_linux_arm64_%GIT_VERSION%.tar"

:: FreeBSD
echo 正在编译 FreeBSD 版本...
set GOOS=freebsd
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_freebsd_386_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_freebsd_386_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_freebsd_386_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_freebsd_386_%GIT_VERSION%.tar"

set GOOS=freebsd
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_freebsd_amd64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_freebsd_amd64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_freebsd_amd64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_freebsd_amd64_%GIT_VERSION%.tar"

set GOOS=freebsd
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_freebsd_arm32_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_freebsd_arm32_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_freebsd_arm32_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_freebsd_arm32_%GIT_VERSION%.tar"

set GOOS=freebsd
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_freebsd_arm64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_freebsd_arm64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_freebsd_arm64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_freebsd_arm64_%GIT_VERSION%.tar"

:: macOS
echo 正在编译 macOS 版本...
set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_darwin_amd64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_darwin_amd64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_darwin_amd64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_darwin_amd64_%GIT_VERSION%.tar"

set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/StellarCore/%BINARY_NAME%" ./cmd/frpc
upx --best --lzma "output/StellarCore/%BINARY_NAME%"
cd output && 7z a -ttar "StellarCore/%VERSION%_darwin_arm64_%GIT_VERSION%.tar" "StellarCore/%BINARY_NAME%" && 7z a -tgzip "StellarCore/%VERSION%_darwin_arm64_%GIT_VERSION%.tar.gz" "StellarCore/%VERSION%_darwin_arm64_%GIT_VERSION%.tar" && cd ..
del "output\StellarCore\%BINARY_NAME%" "output\StellarCore\%VERSION%_darwin_arm64_%GIT_VERSION%.tar"

echo.
echo 编译完成！
echo 输出文件在 output 目录中

endlocal
