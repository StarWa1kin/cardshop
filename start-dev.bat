@echo off
setlocal EnableExtensions

rem Run a single service when this script is called by one of the child windows.
if /i "%~1"=="backend" goto :backend
if /i "%~1"=="user" goto :user
if /i "%~1"=="admin" goto :admin

cd /d "%~dp0"
title Dujiao-Next Launcher

rem A per-user Go install may not be visible to already-running Explorer sessions yet.
where go >nul 2>nul
if errorlevel 1 if exist "%LOCALAPPDATA%\Programs\go\bin\go.exe" (
    set "PATH=%LOCALAPPDATA%\Programs\go\bin;%PATH%"
)

where go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Go was not found in PATH. Please install Go first.
    pause
    exit /b 1
)

where corepack >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Corepack was not found in PATH. Please install Node.js first.
    pause
    exit /b 1
)

if not exist "config.yml" (
    if exist "config.yml.example" (
        copy /y "config.yml.example" "config.yml" >nul
        echo [INFO] Created config.yml from config.yml.example.
    ) else (
        echo [ERROR] config.yml and config.yml.example were not found.
        pause
        exit /b 1
    )
)

echo Starting Dujiao-Next development services...
start "Dujiao Backend - 8080" cmd /k call "%~f0" backend
start "Dujiao User - 5173" cmd /k call "%~f0" user
start "Dujiao Admin - 5174" cmd /k call "%~f0" admin

echo.
echo All service windows have been opened:
echo   Backend:    http://127.0.0.1:8080
echo   User site:  http://127.0.0.1:5173
echo   Admin site: http://127.0.0.1:5174
echo.
echo Close the three service windows to stop all services.
timeout /t 5 >nul
exit /b 0

:backend
cd /d "%~dp0"
title Dujiao Backend - 8080
echo [Backend] Starting on http://127.0.0.1:8080 ...
go run ./cmd/server
goto :service_stopped

:user
cd /d "%~dp0frontend\user"
title Dujiao User - 5173
call :install_frontend_deps
if errorlevel 1 goto :service_failed
echo [User] Starting on http://127.0.0.1:5173 ...
call corepack pnpm run dev
goto :service_stopped

:admin
cd /d "%~dp0frontend\admin"
title Dujiao Admin - 5174
call :install_frontend_deps
if errorlevel 1 goto :service_failed
echo [Admin] Starting on http://127.0.0.1:5174 ...
call corepack pnpm run dev
goto :service_stopped

:install_frontend_deps
if exist "node_modules" exit /b 0
echo [Setup] node_modules not found. Installing dependencies...
call corepack pnpm install
exit /b %errorlevel%

:service_failed
echo.
echo [ERROR] Failed to prepare this service.
pause
exit /b 1

:service_stopped
echo.
echo [INFO] Service stopped. Press any key to close this window.
pause >nul
exit /b 0
