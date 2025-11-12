@echo off
chcp 65001 >nul 2>&1
setlocal EnableDelayedExpansion

set VERSION=0.61.2
set BINARY_NAME_CORE=StellarCore

if exist output (
    rmdir /s /q output
)
mkdir output

where 7z >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 请先安装 7-Zip: https://7-zip.org/
    exit /b 1
)

where upx >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 请先安装 UPX: https://github.com/upx/upx/releases
    exit /b 1
)

set GOOS=windows
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%.exe" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%.exe"
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME_CORE%.exe' -DestinationPath 'output/%BINARY_NAME_CORE%_%VERSION%_windows_386.zip' -Force"
del "output\%BINARY_NAME_CORE%.exe"

set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%.exe" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%.exe"
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME_CORE%.exe' -DestinationPath 'output/%BINARY_NAME_CORE%_%VERSION%_windows_amd64.zip' -Force"
del "output\%BINARY_NAME_CORE%.exe"

set GOOS=windows
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%.exe" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%.exe"
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME_CORE%.exe' -DestinationPath 'output/%BINARY_NAME_CORE%_%VERSION%_windows_arm32.zip' -Force"
del "output\%BINARY_NAME_CORE%.exe"

set GOOS=windows
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%.exe" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%.exe"
powershell -Command "Compress-Archive -Path 'output/%BINARY_NAME_CORE%.exe' -DestinationPath 'output/%BINARY_NAME_CORE%_%VERSION%_windows_arm64.zip' -Force"
del "output\%BINARY_NAME_CORE%.exe"

set GOOS=linux
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_386.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_386.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_386.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_386.tar"

set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_amd64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_amd64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_amd64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_amd64.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=5
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v5.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v5.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v5.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_arm32v5.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=6
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v6.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v6.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v6.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_arm32v6.tar"

set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v7.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v7.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_arm32v7.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_arm32v7.tar"

set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_linux_arm64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_linux_arm64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_linux_arm64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_linux_arm64.tar"

set GOOS=freebsd
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_freebsd_386.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_freebsd_386.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_freebsd_386.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_freebsd_386.tar"

set GOOS=freebsd
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_freebsd_amd64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_freebsd_amd64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_freebsd_amd64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_freebsd_amd64.tar"

set GOOS=freebsd
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm32.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm32.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm32.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_freebsd_arm32.tar"

set GOOS=freebsd
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_freebsd_arm64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_freebsd_arm64.tar"

set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_darwin_amd64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_darwin_amd64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_darwin_amd64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_darwin_amd64.tar"

set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o "output/%BINARY_NAME_CORE%" ./cmd/frps
upx --best --lzma "output/%BINARY_NAME_CORE%"
cd output && 7z a -ttar "%BINARY_NAME_CORE%_%VERSION%_darwin_arm64.tar" "%BINARY_NAME_CORE%" && 7z a -tgzip "%BINARY_NAME_CORE%_%VERSION%_darwin_arm64.tar.gz" "%BINARY_NAME_CORE%_%VERSION%_darwin_arm64.tar" && cd ..
del "output\%BINARY_NAME_CORE%" "output\%BINARY_NAME_CORE%_%VERSION%_darwin_arm64.tar"

echo 构建完成，输出文件在 output 目录

endlocal
