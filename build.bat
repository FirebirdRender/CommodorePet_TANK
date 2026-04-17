@echo off
SETLOCAL EnableDelayedExpansion

:menu
echo.
echo Tank Game Build Script
echo 1. wasm          - Build WASM client
echo 2. server        - Build Go server
echo 3. build-all     - Build both WASM and Server
echo 4. dev           - Run server in dev mode
echo 5. test          - Run all tests with race detector
echo 6. test-headless - Run headless-safe tests
echo 7. test-e2e      - Run Playwright E2E tests
echo 8. test-all     - Run all test suites
echo 9. clean         - Remove build artifacts
echo 0. exit
set /p choice="Select an option (0-9): "

if "%choice%"=="1" goto wasm
if "%choice%"=="2" goto server
if "%choice%"=="3" goto build_all
if "%choice%"=="4" goto dev
if "%choice%"=="5" goto test
if "%choice%"=="6" goto test_headless
if "%choice%"=="7" goto test_e2e
if "%choice%"=="8" goto test_all
if "%choice%"=="9" goto clean
if "%choice%"=="0" exit
goto menu

:copy_font
if not exist "internal\assets\fonts" mkdir "internal\assets\fonts"
copy /Y "tank_game\assets\fonts\PetMe64.ttf" "internal\assets\fonts\"
exit /b

:copy_web_fonts
if not exist "web\fonts" mkdir "web\fonts"
copy /Y "docs\FONTS\PetMe.ttf" "web\fonts\PetMe.ttf"
exit /b

:copy_wasm_exec
for /f "usebackq tokens=*" %%i in (`go env GOROOT`) do set GOROOT_PATH=%%i
copy /Y "%GOROOT_PATH%\lib\wasm\wasm_exec.js" "web\wasm_exec.js"
exit /b

:wasm
call :copy_font
call :copy_wasm_exec
call :copy_web_fonts
echo Building WASM...
set GOOS=js
set GOARCH=wasm
go build -trimpath -ldflags "-s -w" -o web/game.wasm ./cmd/client/
set GOOS=
set GOARCH=
echo WASM build complete.
pause
goto menu

:server
if not exist "bin" mkdir "bin"
echo Building Server...
go build -o bin/tank-server.exe ./cmd/server/
echo Server build complete: bin\tank-server.exe
pause
goto menu

:build_all
call :wasm
call :server
echo All builds complete.
pause
goto menu

:dev
call :copy_wasm_exec
echo Starting dev server (Ctrl+C to stop)...
go run ./cmd/server/ -addr :8080 -dir web
goto menu

:test
echo Running all tests with race detector...
go test -race ./... -count=1
echo Tests complete.
pause
goto menu

:test_headless
echo Running headless-safe tests...
go test -race ./client/... -count=1 -run "TestNewGameState|TestApply|TestReset|TestKeyToDir|TestCellGlyphs|TestBarrel|TestGlyphCache|TestNewRenderer|TestColor|TestDimension|TestNewGameSetsPlayerName|TestExportGameStateNativeNoop|TestAnimTick"
echo Headless tests complete.
pause
goto menu

:test_e2e
echo Building before E2E tests...
call :build_all
echo Running Playwright E2E tests...
cd test\e2e
call npm install
call npx playwright install chromium
call npx playwright test
cd ..\..
echo E2E tests complete.
pause
goto menu

:test_all
echo Running all test suites...
echo [1/3] Go tests...
go test -race ./... -count=1
echo [2/3] Headless tests...
go test -race ./client/... -count=1 -run "TestNewGameState|TestApply|TestReset|TestKeyToDir|TestCellGlyphs|TestBarrel|TestGlyphCache|TestNewRenderer|TestColor|TestDimension|TestNewGameSetsPlayerName|TestExportGameStateNativeNoop|TestAnimTick"
echo [3/3] Playwright E2E tests...
cd test\e2e
call npm install
call npx playwright install chromium
call npx playwright test
cd ..\..
echo All test suites complete.
pause
goto menu

:clean
if exist "web\game.wasm" del "web\game.wasm"
if exist "bin" rd /s /q "bin"
if exist "web\fonts" rd /s /q "web\fonts"
echo Cleaned build artifacts.
pause
goto menu