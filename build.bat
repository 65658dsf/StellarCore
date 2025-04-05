@echo off
setlocal EnableDelayedExpansion

:: 版本号
set VERSION=1.1.0
set BINARY_NAME=StellarFrpc

:: 创建输出目录
if not exist output mkdir output

:: 检查是否安装了 7-Zip
where 7z >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 错误：请先安装 7-Zip！
    echo 下载地址：https://7-zip.org/
    exit /b 1
)

:: Windows
echo 正在编译 Windows 版本...
set GOOS=windows
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%.exe" ./cmd/frpc
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME%.exe' -DestinationPath 'output/%BINARY_NAME%_%VERSION%_windows_386.zip' -Force"
del "output\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%.exe" ./cmd/frpc
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME%.exe' -DestinationPath 'output/%BINARY_NAME%_%VERSION%_windows_amd64.zip' -Force"
del "output\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%.exe" ./cmd/frpc
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME%.exe' -DestinationPath 'output/%BINARY_NAME%_%VERSION%_windows_arm32.zip' -Force"
del "output\%BINARY_NAME%.exe"

set GOOS=windows
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%.exe" ./cmd/frpc
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME%.exe' -DestinationPath 'output/%BINARY_NAME%_%VERSION%_windows_arm64.zip' -Force"
del "output\%BINARY_NAME%.exe"

:: Linux
echo 正在编译 Linux 版本...
set GOOS=linux
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_386.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_386.tar.gz" "%BINARY_NAME%_%VERSION%_linux_386.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_386.tar"

set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_amd64.tar.gz" "%BINARY_NAME%_%VERSION%_linux_amd64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_amd64.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=5
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_arm32v5.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_arm32v5.tar.gz" "%BINARY_NAME%_%VERSION%_linux_arm32v5.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_arm32v5.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=6
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_arm32v6.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_arm32v6.tar.gz" "%BINARY_NAME%_%VERSION%_linux_arm32v6.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_arm32v6.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_arm32v7.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_arm32v7.tar.gz" "%BINARY_NAME%_%VERSION%_linux_arm32v7.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_arm32v7.tar"

set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_linux_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_linux_arm64.tar.gz" "%BINARY_NAME%_%VERSION%_linux_arm64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_linux_arm64.tar"

:: FreeBSD
echo 正在编译 FreeBSD 版本...
set GOOS=freebsd
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_freebsd_386.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_freebsd_386.tar.gz" "%BINARY_NAME%_%VERSION%_freebsd_386.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_freebsd_386.tar"

set GOOS=freebsd
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_freebsd_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_freebsd_amd64.tar.gz" "%BINARY_NAME%_%VERSION%_freebsd_amd64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_freebsd_amd64.tar"

set GOOS=freebsd
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_freebsd_arm32.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_freebsd_arm32.tar.gz" "%BINARY_NAME%_%VERSION%_freebsd_arm32.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_freebsd_arm32.tar"

set GOOS=freebsd
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_freebsd_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_freebsd_arm64.tar.gz" "%BINARY_NAME%_%VERSION%_freebsd_arm64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_freebsd_arm64.tar"

:: macOS
echo 正在编译 macOS 版本...
set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_darwin_amd64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_darwin_amd64.tar.gz" "%BINARY_NAME%_%VERSION%_darwin_amd64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_darwin_amd64.tar"

set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME%" ./cmd/frpc
cd output && 7z a -ttar "%BINARY_NAME%_%VERSION%_darwin_arm64.tar" "%BINARY_NAME%" && 7z a -tgzip "%BINARY_NAME%_%VERSION%_darwin_arm64.tar.gz" "%BINARY_NAME%_%VERSION%_darwin_arm64.tar" && cd ..
del "output\%BINARY_NAME%" "output\%BINARY_NAME%_%VERSION%_darwin_arm64.tar"

echo.
echo 编译完成！
echo 输出文件在 output 目录中

endlocal