package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/pkg/browser"
	"github.com/sqweek/dialog"

	"pveRH-mod-manager/internal"
)

//go:embed web
var webFS embed.FS

//go:embed gamefile.json
var gameFileJSON []byte

//go:embed mod.json
var modInfoJSON []byte

var (
	configManager *internal.ConfigManager
	gamePath      string
	modLibPath    string
	modOperator   *internal.ModOperator
)

func configFilePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exePath), "config.json")
}

func loadConfig() {
	configManager = internal.NewConfigManager(configFilePath())
	configManager.Load()

	cfg := configManager.GetConfig()
	if len(cfg.GamePaths) > 0 {
		gamePath = cfg.GamePaths[0].Path
	}
	if len(cfg.ModPaths) > 0 {
		modLibPath = cfg.ModPaths[0]
	}
}

func saveConfig() {
	configManager.Save()
}

func initModOperator() {
	if gamePath != "" && modLibPath != "" {
		modOperator = internal.NewModOperator(gamePath, modLibPath)
	}
}

func selectFolder(title string) (string, error) {
	return dialog.Directory().Title(title).Browse()
}

func addGamePath(path string) {
	configManager.AddGamePath(path)
	gamePath = path
	saveConfig()
}

func addModLibPath(path string) {
	configManager.AddModPath(path)
	modLibPath = path
	saveConfig()
}

func main() {
	internal.SetEmbeddedGameFileData(gameFileJSON)
	internal.SetEmbeddedModInfoData(modInfoJSON)

	loadConfig()
	initModOperator()

	r := mux.NewRouter()
	api := r.PathPrefix("/api").Subrouter()

	// 配置
	api.HandleFunc("/config", handleGetConfig).Methods("GET")

	// 游戏目录和版本
	api.HandleFunc("/switch-game", handleSwitchGame).Methods("POST")
	api.HandleFunc("/switch-version", handleSwitchVersion).Methods("POST")
	api.HandleFunc("/add-game-path", handleAddGamePath).Methods("POST")
	api.HandleFunc("/add-mod-path", handleAddModPath).Methods("POST")

	// BepInEx
	api.HandleFunc("/check-bepinex", handleCheckBepInEx).Methods("GET")
	api.HandleFunc("/install-bepinex", handleInstallBepInEx).Methods("POST")
	api.HandleFunc("/remove-bepinex", handleRemoveBepInEx).Methods("POST")

	// MOD 管理
	api.HandleFunc("/mods", handleGetMods).Methods("GET")
	api.HandleFunc("/install-mod", handleInstallMod).Methods("POST")
	api.HandleFunc("/remove-mod-by-name", handleRemoveModByName).Methods("POST")

	// 目录操作
	api.HandleFunc("/open-folder", handleOpenFolder).Methods("POST")

	// 静态文件服务
	webDir, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal("嵌入 web 目录失败: ", err)
	}
	r.PathPrefix("/").Handler(http.FileServer(http.FS(webDir)))

	go func() {
		browser.OpenURL("http://localhost:8080")
	}()

	log.Println("服务已启动：http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// ========== 配置 ==========

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := configManager.GetConfig()

	// 从当前游戏目录关联的版本
	currentVersion := ""
	for _, gp := range cfg.GamePaths {
		if gp.Path == gamePath {
			currentVersion = gp.Version
			break
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"game_paths":      cfg.GamePaths,
		"mod_paths":       cfg.ModPaths,
		"download_path":   cfg.DownloadPath,
		"current_game":    gamePath,
		"current_modlib":  modLibPath,
		"current_version": currentVersion,
		"versions":        configManager.GetVersions(),
	})
}

// ========== 游戏目录和版本 ==========

func handleSwitchGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		http.Error(w, "游戏目录不存在", http.StatusBadRequest)
		return
	}

	gamePath = req.Path
	initModOperator()
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleSwitchVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	paths := configManager.GetGamePathsByVersion(req.Version)
	if len(paths) == 0 {
		http.Error(w, "未找到该版本的游戏目录", http.StatusNotFound)
		return
	}

	gamePath = paths[0]
	initModOperator()
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "game_path": gamePath})
}

func handleAddGamePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}
	path, err := selectFolder("选择游戏根目录")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"path": ""})
		return
	}
	addGamePath(path)
	initModOperator()
	json.NewEncoder(w).Encode(map[string]string{"path": path})
}

func handleAddModPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}
	path, err := selectFolder("选择 Mod 库文件夹")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"path": ""})
		return
	}
	addModLibPath(path)
	initModOperator()
	json.NewEncoder(w).Encode(map[string]string{"path": path})
}

// ========== BepInEx ==========

func handleCheckBepInEx(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" {
		http.Error(w, "请先选择游戏目录", http.StatusBadRequest)
		return
	}
	installed, missing := internal.CheckBepInExFiles(gamePath)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"installed": installed,
		"missing":   missing,
	})
}

func handleInstallBepInEx(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "目录未设置", http.StatusBadRequest)
		return
	}
	if err := internal.InstallBepInEx(gamePath, modLibPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleRemoveBepInEx(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" {
		http.Error(w, "请先选择游戏目录", http.StatusBadRequest)
		return
	}
	if err := internal.RemoveBepInEx(gamePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ========== MOD 管理 ==========

func handleGetMods(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "请先设置目录", http.StatusBadRequest)
		return
	}
	if modOperator == nil {
		http.Error(w, "Mod 操作器未初始化", http.StatusInternalServerError)
		return
	}

	status, err := modOperator.GetModStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(status)
}

func handleInstallMod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	mods, _ := internal.ScanModLibrary(modLibPath)
	var target internal.AvailableMod
	for _, m := range mods {
		if m.Name == req.Name && !m.IsZip {
			target = m
			break
		}
	}
	if target.Name == "" {
		http.Error(w, "Mod 未找到", http.StatusNotFound)
		return
	}

	if err := internal.InstallMod(target, gamePath, modLibPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleRemoveModByName(w http.ResponseWriter, r *http.Request) {
	if modOperator == nil {
		http.Error(w, "请先设置游戏目录和 Mod 库", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	if err := modOperator.RemoveModByChineseName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ========== 目录操作 ==========

func handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	var path string
	switch req.Type {
	case "game":
		path = gamePath
	case "mod":
		path = modLibPath
	case "installed":
		if gamePath != "" {
			path = filepath.Join(gamePath, "BepInEx", "plugins")
		}
	default:
		http.Error(w, "无效的目录类型", http.StatusBadRequest)
		return
	}

	if path == "" {
		http.Error(w, "目录未设置", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("explorer", path)
	if err := cmd.Start(); err != nil {
		http.Error(w, "打开目录失败", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
