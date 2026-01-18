@echo off
echo Cleaning build artifacts...
if exist "build\bin" rmdir /s /q "build\bin"
if exist "frontend\dist" rmdir /s /q "frontend\dist"
echo Clean complete. Building...
call build.bat

if exist "build\bin\DINKIssTyleChat.exe" (
    echo Copying assets...
    rem xcopy /E /I /Y "assets" "build\bin\assets"
    xcopy /E /I /Y "frontend" "build\bin\frontend"
    copy /Y "onnxruntime\onnxruntime.dll" "build\bin\" 2>nul
    copy /Y "users.json" "build\bin\" 2>nul
    copy /Y "config.json" "build\bin\" 2>nul
    echo Build success!
) else (
    echo Build failed!
)
