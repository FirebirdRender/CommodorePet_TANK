@echo off
SETLOCAL EnableDelayedExpansion

:menu
echo.
echo Tank Game Build Script
echo 1. wasm          - Build WASM client
echo 2. server        - Build Go server
echo 3. bot           - Build reference bot
echo 4. build-all     - Build WASM, Server, and Bot
echo 5. dev           - Run server in dev mode
echo 6. test          - Run all tests with race detector
echo 7. test-headless - Run headless-safe tests
echo 8. test-e2e      - Run Playwright E2E tests
echo 9. test-all      - Run all test suites
echo c. clean         - Remove build artifacts
echo 0. exit
set /p choice="Select an option: "

if "%choice%"=="1" call :do_wasm && pause & goto menu
if "%choice%"=="2" call :do_server && pause & goto menu
if "%choice%"=="3" call :do_bot && pause & goto menu
if "%choice%"=="4" call :do_build_all && pause & goto menu
if "%choice%"=="5" call :do_dev & goto menu
if "%choice%"=="6" call :do_test && pause & goto menu
if "%choice%"=="7" call :do_test_headless && pause & goto menu
if "%choice%"=="8" call :do_test_e2e && pause & goto menu
if "%choice%"=="9" call :do_test_all && pause & goto menu
if /i "%choice%"=="c" call :do_clean && pause & goto menu
if "%choice%"=="0" exit
goto menu

rem --- Helper subroutines (always exit /b so they return to caller) ---

:ensure_bin
if not exist "bin" mkdir "bin"
exit /b

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

rem --- Build actions ---

:do_wasm
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
exit /b

:do_server
call :ensure_bin
echo Building Server...
go build -o bin\tank-server.exe ./cmd/server/
echo Server build complete: bin\tank-server.exe
exit /b

:do_bot
call :ensure_bin
echo Building Bot...
go build -o bin\tank-bot.exe ./cmd/bot-go/
echo Bot build complete: bin\tank-bot.exe
exit /b

:do_build_all
call :do_wasm
call :do_server
call :do_bot
echo All builds complete.
exit /b

rem --- Dev / Test actions ---

:do_dev
call :copy_wasm_exec
echo Starting dev server (Ctrl+C to stop)...
go run ./cmd/server/ -addr :8080 -dir web
exit /b

:do_test
echo Running all tests with race detector...
go test -race ./... -count=1
echo Tests complete.
exit /b

:do_test_headless
echo Running headless-safe tests...
go test -race ./client/... -count=1 -run "TestNewGameState|TestApply|TestReset|TestKeyToDir|TestCellGlyphs|TestBarrel|TestGlyphCache|TestNewRenderer|TestColor|TestDimension|TestNewGameSetsPlayerName|TestExportGameStateNativeNoop|TestAnimTick"
echo Headless tests complete.
exit /b

:do_test_e2e
echo Building before E2E tests...
call :do_build_all
echo Running Playwright E2E tests...
cd test\e2e
call npm install
call npx playwright install chromium
call npx playwright test
cd ..\..
echo E2E tests complete.
exit /b

:do_test_all
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
exit /b

:do_clean
if exist "web\game.wasm" del "web\game.wasm"
if exist "bin" rd /s /q "bin"
if exist "web\fonts" rd /s /q "web\fonts"
echo Cleaned build artifacts.
exit /b