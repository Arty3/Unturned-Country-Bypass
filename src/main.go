package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func fatalError(message string, err error) {
	fmt.Printf("ERROR: %s\n%v\n", message, err)
	fmt.Print("\nPress enter to exit ... ")
	fmt.Scanln()
	os.Exit(1)
}

type Module struct {
	IsEnabled  bool       `json:"IsEnabled"`
	Name       string     `json:"Name"`
	Version    string     `json:"Version"`
	Assemblies []Assembly `json:"Assemblies"`
}

type Assembly struct {
	Path            string `json:"Path"`
	Role            string `json:"Role"`
	LoadAsByteArray bool   `json:"Load_As_Byte_Array"`
}

func main() {
	basePath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fatalError("Failed to get base path", err)
	}

	osPlatform := runtime.GOOS
	fmt.Printf("Detected operating system: %s\n", osPlatform)

	if osPlatform != "windows" {
		fatalError("Unsupported operating system.", errors.New("this script only supports Windows"))
	}

	fmt.Println("Verifying administrator privileges...")

	if !isAdmin() {
		fatalError("This script must be run as an administrator.", errors.New("insufficient privileges"))
	}

	fmt.Println("Successfully verified administrator privileges.")

	fmt.Println("Locating unturned game files...")

	untPath, err := findUnturnedGamePath()
	if err != nil {
		fatalError("Failed to find Unturned game files", err)
	}

	fmt.Printf("Located Unturned game files: %s\n", untPath)

	mdlPath := filepath.Join(untPath, "Modules", "BypassCountryRestrictions")

	fileInfo, err := os.Stat(untPath)
	if err != nil || !fileInfo.IsDir() {
		fatalError("Unturned game files path is not a directory.", err)
	}

	fmt.Println("Successfully located Unturned directory.")

	tempFile := filepath.Join(untPath, "temp_permission_check")
	f, err := os.Create(tempFile)
	if err != nil {
		fatalError("Missing permissions to access the unturned directory.", err)
	}
	f.Close()
	os.Remove(tempFile)

	fmt.Println("Successfully verified permissions.")

	dataPath := filepath.Join(basePath, "data")
	if !directoryExists(dataPath) {
		fmt.Println("Creating data directory...")
		if err := os.Mkdir(dataPath, 0777); err != nil {
			fatalError("Failed to create data directory", err)
		}
	}

	if !directoryExists(mdlPath) {
		fmt.Println("Creating module directory...")
		if err := os.Mkdir(mdlPath, 0777); err != nil {
			fatalError("Failed to create module directory", err)
		}
	}

	fmt.Println("Writing binary .dll file to module...")

	binPath := filepath.Join(mdlPath, "bin")
	if !directoryExists(binPath) {
		srcBinPath := filepath.Join(basePath, "bin")
		if !directoryExists(srcBinPath) || !fileExists(filepath.Join(srcBinPath, "BypassCountryRestrictions.dll")) {
			fatalError("No binary detected to copy.", errors.New("missing binary files"))
		}

		err := copyDir(srcBinPath, binPath)
		if err != nil {
			fatalError(fmt.Sprintf("Failed to write binary .dll file to module: %v", err), err)
		}

		fmt.Println("Successfully wrote binary .dll file.")
	} else {
		fmt.Println("Found existing binary .dll file. Skipping...")
	}

	fmt.Println("Creating client shutdown flag...")

	flagPath := filepath.Join(mdlPath, "Flag")
	flagFile, err := os.OpenFile(flagPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fatalError("Failed to create client shutdown flag", err)
	}
	flagFile.Close()

	fmt.Println("Successfully created client shutdown flag.")

	fmt.Println("Writing language .dat file...")

	datPath := filepath.Join(mdlPath, "English.dat")
	datFile, err := os.OpenFile(datPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fatalError("Failed to create language file", err)
	}

	_, err = datFile.WriteString("Name BypassCountryRestrictions\nDescription Module which bypasses all country restrictions.")
	if err != nil {
		datFile.Close()
		fatalError("Failed to write language file content", err)
	}
	datFile.Close()

	fmt.Println("Successfully wrote language .dat file.")

	fmt.Println("Writing .module json file...")

	module := Module{
		IsEnabled: true,
		Name:      "BypassCountryRestrictions",
		Version:   "1.0.0.0",
		Assemblies: []Assembly{
			{
				Path:            "/bin/BypassCountryRestrictions.dll",
				Role:            "Both_Optional",
				LoadAsByteArray: false,
			},
		},
	}

	moduleJSON, err := json.MarshalIndent(module, "", "    ")
	if err != nil {
		fatalError("Failed to create module JSON", err)
	}

	fmt.Printf("Module data:\n %s\n", string(moduleJSON))

	fmt.Println("Verifying assemblies...")

	for _, asm := range module.Assemblies {
		asmPath := filepath.Join(mdlPath, strings.ReplaceAll(asm.Path, "/", string(os.PathSeparator)))
		if !fileExists(asmPath) {
			fatalError(fmt.Sprintf("Failed to find assembly for %s", asm.Path), errors.New("missing assembly"))
		}
	}

	fmt.Println("Successfully verified assemblies.")

	fmt.Println("Writing content...")

	moduleFilePath := filepath.Join(mdlPath, "BypassCountryRestrictions.module")
	moduleFile, err := os.OpenFile(moduleFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fatalError("Failed to create module file", err)
	}

	_, err = moduleFile.Write(moduleJSON)
	if err != nil {
		moduleFile.Close()
		fatalError("Failed to write module file content", err)
	}
	moduleFile.Close()

	fmt.Println("Successfully wrote .module json file.")
	fmt.Println("Successfully setup the module.")

	fmt.Println("Launching Unturned (without BattlEye)...")
	fmt.Println("Disabling BattlEye...")

	beIniPath := filepath.Join(untPath, "BattlEye", "BELauncher.ini")
	beContent, err := os.ReadFile(beIniPath)
	if err != nil {
		fatalError("Failed to read BattlEye configuration", err)
	}

	fmt.Printf("BattlEye launch configuration:\n%s\n", string(beContent))

	fmt.Println("Modifying sys argv argument...")

	var modifiedLines []string
	for _, line := range strings.Split(string(beContent), "\n") {
		if strings.HasPrefix(line, "BEArg=") {
			line = "BEArg="
		}
		modifiedLines = append(modifiedLines, line)
	}

	modifiedContent := strings.Join(modifiedLines, "\n")

	fmt.Println("Replaced BEArg to be NULL.")

	err = os.WriteFile(beIniPath, []byte(modifiedContent), 0666)
	if err != nil {
		fatalError("Failed to write modified BattlEye configuration", err)
	}

	fmt.Println("Successfully disabled BattlEye.")

	cmd := []string{"C:\\Program Files (x86)\\Steam\\Steam.exe", "-applaunch", "304930"}

	fmt.Printf("Executing command: %s\n", strings.Join(cmd, " "))

	fmt.Println("Launching...")

	steamCmd := exec.Command(cmd[0], cmd[1:]...)
	err = steamCmd.Start()
	if err != nil {
		fatalError("Failed to launch Unturned", err)
	}

	fmt.Println("Successfully launched session.")

	fmt.Println("Awaiting client shutdown...")
	fmt.Println("DO NOT CLOSE THIS WINDOW.")

	clientIsAlive := true

	for clientIsAlive {
		time.Sleep(5 * time.Second)

		flagContent, err := os.ReadFile(flagPath)
		if err == nil && strings.TrimSpace(string(flagContent)) == "true" {
			clientIsAlive = false
		}
	}

	fmt.Println("Detected client shutdown.")

	time.Sleep(3 * time.Second)

	fmt.Println("Destroying module...")

	err = os.RemoveAll(mdlPath)
	if err != nil {
		fatalError("Failed to destroy module", err)
	}

	fmt.Println("Successfully destroyed module.")

	fmt.Println("Re-enabling BattlEye...")

	beContent, err = os.ReadFile(beIniPath)
	if err != nil {
		fatalError("Failed to read BattlEye configuration", err)
	}

	fmt.Println("Modifying sys argv argument...")

	modifiedLines = []string{}
	for _, line := range strings.Split(string(beContent), "\n") {
		if strings.HasPrefix(line, "BEArg=") {
			line = "BEArg=-BattlEye"
		}
		modifiedLines = append(modifiedLines, line)
	}

	modifiedContent = strings.Join(modifiedLines, "\n")

	fmt.Println("Replaced BEArg to be -BattlEye.")

	err = os.WriteFile(beIniPath, []byte(modifiedContent), 0666)
	if err != nil {
		fatalError("Failed to write modified BattlEye configuration", err)
	}

	fmt.Println("\nFinished.")
	fmt.Print("\nPress enter to continue ... ")
	fmt.Scanln()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0777); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

func isAdmin() bool {
	if runtime.GOOS == "windows" {
		var token windows.Token
		current := windows.CurrentProcess()
		err := windows.OpenProcessToken(current, windows.TOKEN_QUERY, &token)
		if err != nil {
			return false
		}
		defer token.Close()

		var isElevated uint32
		var outLen uint32
		err = windows.GetTokenInformation(token, windows.TokenElevation,
			(*byte)(unsafe.Pointer(&isElevated)),
			uint32(unsafe.Sizeof(isElevated)), &outLen)

		if err != nil {
			return false
		}

		return isElevated != 0
	}

	return false
}

// Steam game path finder functionality (ported from C++)
const (
	cacheFile      = ".gamepath"
	regexPattern   = `"path"\s*"([^"]+)"`
	allowPathCache = true
)

var registryKeys = []string{
	"SOFTWARE\\WOW6432Node\\Valve\\Steam",
	"SOFTWARE\\Valve\\Steam",
}

func getSteamPathFromRegistry() (string, error) {
	for _, regKey := range registryKeys {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, regKey, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		defer k.Close()

		steamPath, _, err := k.GetStringValue("InstallPath")
		if err == nil {
			return steamPath, nil
		}
	}
	return "", errors.New("steam path not found in registry")
}

func parseLibraryFolders(baseSteamPath string) []string {
	libraryFoldersFile := filepath.Join(baseSteamPath, "steamapps", "libraryfolders.vdf")
	libraryPaths := []string{baseSteamPath}

	if !fileExists(libraryFoldersFile) {
		return libraryPaths
	}

	content, err := os.ReadFile(libraryFoldersFile)
	if err != nil {
		fmt.Printf("Error reading library folders: %v\n", err)
		return libraryPaths
	}

	re := regexp.MustCompile(regexPattern)
	matches := re.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			libraryPath := match[1]
			libraryPath = filepath.FromSlash(libraryPath)

			commonPath := filepath.Join(libraryPath, "steamapps", "common")
			if directoryExists(commonPath) {
				libraryPaths = append(libraryPaths, libraryPath)
			}
		}
	}

	return libraryPaths
}

func findGameDirectory(game string) (string, error) {
	steamPath, err := getSteamPathFromRegistry()
	if err != nil {
		return "", err
	}

	libraryPaths := parseLibraryFolders(steamPath)

	for _, libraryPath := range libraryPaths {
		commonPath := filepath.Join(libraryPath, "steamapps", "common")
		if !directoryExists(commonPath) {
			continue
		}

		entries, err := os.ReadDir(commonPath)
		if err != nil {
			fmt.Printf("Failed to access directory %s: %v\n", commonPath, err)
			continue
		}

		for _, entry := range entries {
			if entry.Name() == game {
				gamePath := filepath.Join(commonPath, game)
				exePath := filepath.Join(gamePath, game+".exe")
				if fileExists(exePath) {
					return gamePath, nil
				}
			}
		}
	}

	return "", fmt.Errorf("game directory for %s not found", game)
}

func writeCachePath(path string) bool {
	if !allowPathCache {
		return false
	}

	err := os.WriteFile(cacheFile, []byte(path), 0666)
	if err != nil {
		fmt.Printf("Error writing cache: %v\n", err)
		return false
	}
	return true
}

func readCachePath() (string, bool) {
	if !allowPathCache {
		return "", false
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return "", false
	}

	return string(data), true
}

func findUnturnedGamePath() (string, error) {
	const gameName = "Unturned"

	if allowPathCache {
		if path, found := readCachePath(); found {
			if directoryExists(path) {
				return path, nil
			}
		}
	}

	path, err := findGameDirectory(gameName)
	if err != nil {
		return "", err
	}

	if allowPathCache {
		writeCachePath(path)
	}

	return path, nil
}
