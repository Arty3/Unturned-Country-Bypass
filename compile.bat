@echo off

echo Compiling Unturned Country Restriction Bypass Tool...

:: Check if Go is installed
where go >nul 2>nul
if %errorlevel% NEQ 0 (
	echo ERROR: Go is not installed or not in your PATH.
	echo Please install Go from https://golang.org/dl/
	pause
	exit /b 1
)

:: Create a temporary directory for the build
if not exist build mkdir build

:: Compile the program
go build -o build/loader.exe ./src/main.go

if %errorlevel% NEQ 0 (
	echo ERROR: Compilation failed.
	pause
	exit /b %errorlevel%
)

:: Copy necessary files to the build directory
echo Copying required files...

:: Copy bin directory if it exists
if exist bin (
	if not exist build\bin mkdir build\bin
	xcopy /E /I /Y bin build\bin >nul
) else (
	echo WARNING: bin directory not found. Make sure to create it with BypassCountryRestrictions.dll before running the program.
)

echo.
echo Build completed successfully!
echo The executable and required files are in the "build" directory.
echo.
echo To run the program: 
echo 1. Navigate to the "build" directory
echo 2. Run "loader.exe" as administrator
echo.

pause
exit /b 0
