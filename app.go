package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
)

//go:embed frontend/*
var frontend embed.FS

// ===== 应用状态 =====

var appCfg *Config

// ===== 入口 =====

// StartServer 启动 HTTP 服务器并打开浏览器
func StartServer(cfg *Config) error {
	appCfg = cfg

	// 前端静态文件
	frontendFS, err := fs.Sub(frontend, "frontend")
	if err != nil {
		return fmt.Errorf("加载前端文件失败: %w", err)
	}
	http.Handle("/", http.FileServer(http.FS(frontendFS)))

	// API 路由
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/versions", handleVersions)
	http.HandleFunc("/api/bepinex/check", handleCheckBepInEx)
	http.HandleFunc("/api/bepinex/install", handleInstallBepInEx)
	http.HandleFunc("/api/bepinex/uninstall", handleUninstallBepInEx)
	http.HandleFunc("/api/mods/local", handleLocalMods)
	http.HandleFunc("/api/mods/installed", handleInstalledMods)
	http.HandleFunc("/api/mods/install", handleInstallMod)
	http.HandleFunc("/api/mods/uninstall", handleUninstallMod)
	http.HandleFunc("/api/mods/uninstall-all", handleUninstallAllMods)
	http.HandleFunc("/api/modifier/find", handleFindModifier)
	http.HandleFunc("/api/modifier/install", handleInstallModifier)
	http.HandleFunc("/api/online/versions", handleOnlineVersions)
	http.HandleFunc("/api/online/authors", handleOnlineAuthors)
	http.HandleFunc("/api/online/mods", handleOnlineMods)
	http.HandleFunc("/api/online/install", handleOnlineInstall)
	http.HandleFunc("/api/open-dir", handleOpenDir)
	http.HandleFunc("/api/select-folder", handleSelectFolder)

	addr := "127.0.0.1:19527"
	go openBrowser("http://" + addr)

	fmt.Printf("服务器启动: http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}

// openBrowser 打开默认浏览器
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// ===== JSON 响应工具 =====

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ===== API 处理器 =====

func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, appCfg)
	case http.MethodPost:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, "解析请求失败: "+err.Error(), 400)
			return
		}
		if err := SaveConfig(&cfg); err != nil {
			writeError(w, "保存配置失败: "+err.Error(), 500)
			return
		}
		appCfg = &cfg
		fmt.Printf("[配置] 保存: game=%d, mod=%d\n", len(cfg.GamePath), len(cfg.ModPath))
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		writeError(w, "不支持的方法", 405)
	}
}

func handleVersions(w http.ResponseWriter, r *http.Request) {
	versions := DetectVersions(appCfg.ModPath)
	fmt.Printf("[版本] 检测到 %d 个: %v\n", len(versions), versions)
	writeJSON(w, versions)
}

func handleCheckBepInEx(w http.ResponseWriter, r *http.Request) {
	gamePath := r.URL.Query().Get("path")
	if gamePath == "" {
		writeError(w, "缺少 path 参数", 400)
		return
	}
	installed, err := CheckBepInEx(gamePath)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	fmt.Printf("[BepInEx] 检测: %q → %v\n", gamePath, installed)
	writeJSON(w, map[string]bool{"installed": installed})
}

func handleInstallBepInEx(w http.ResponseWriter, r *http.Request) {
	gamePath := r.URL.Query().Get("path")
	if gamePath == "" {
		writeError(w, "缺少 path 参数", 400)
		return
	}
	fmt.Printf("[BepInEx] 安装 → %q\n", gamePath)
	if err := InstallBepInEx(gamePath, appCfg.ModPath); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleUninstallBepInEx(w http.ResponseWriter, r *http.Request) {
	gamePath := r.URL.Query().Get("path")
	if gamePath == "" {
		writeError(w, "缺少 path 参数", 400)
		return
	}
	fmt.Printf("[BepInEx] 卸载 ← %q\n", gamePath)
	if err := UninstallBepInEx(gamePath); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleLocalMods(w http.ResponseWriter, r *http.Request) {
	modPath := r.URL.Query().Get("modPath")
	version := r.URL.Query().Get("version")
	if modPath == "" || version == "" {
		writeError(w, "缺少 modPath 或 version 参数", 400)
		return
	}
	mods, err := ScanLocalMods(modPath, version)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	total := 0
	for _, items := range mods {
		total += len(items)
	}
	fmt.Printf("[MOD] 扫描: version=%s → %d 个\n", version, total)
	writeJSON(w, mods)
}

func handleInstalledMods(w http.ResponseWriter, r *http.Request) {
	gamePath := r.URL.Query().Get("path")
	if gamePath == "" {
		writeError(w, "缺少 path 参数", 400)
		return
	}
	mods, err := ScanInstalledMods(gamePath)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	total := 0
	for _, items := range mods {
		total += len(items)
	}
	fmt.Printf("[MOD] 已安装: %q → %d 个\n", gamePath, total)
	writeJSON(w, mods)
}

// installReq 安装/操作请求，game_path 放在 body 中避免 URL 编码问题
type installReq struct {
	GamePath string       `json:"game_path"`
	Item     LocalModItem `json:"item"`
}

type installModifierReq struct {
	GamePath string       `json:"game_path"`
	Pack     ModifierPack `json:"pack"`
}

type installOnlineReq struct {
	GamePath string        `json:"game_path"`
	Info     ServerModInfo `json:"info"`
}

func handleInstallMod(w http.ResponseWriter, r *http.Request) {
	var req installReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "解析请求失败: "+err.Error(), 400)
		return
	}
	fmt.Printf("[安装MOD] gamePath=%q source=%q\n", req.GamePath, req.Item.SourcePath)
	if err := InstallLocalMod(req.Item, req.GamePath); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleUninstallMod(w http.ResponseWriter, r *http.Request) {
	var item LocalModItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, "解析请求失败: "+err.Error(), 400)
		return
	}
	fmt.Printf("[卸载MOD] source=%q\n", item.SourcePath)
	if err := UninstallMod(item); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleUninstallAllMods(w http.ResponseWriter, r *http.Request) {
	gamePath := r.URL.Query().Get("path")
	fmt.Printf("[MOD] 全部卸载: %q\n", gamePath)
	if err := UninstallAllMods(gamePath); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleFindModifier(w http.ResponseWriter, r *http.Request) {
	modPath := r.URL.Query().Get("modPath")
	version := r.URL.Query().Get("version")
	pack, err := FindModifier(modPath, version)
	if err != nil {
		fmt.Printf("[修改器] 未找到: version=%s → %v\n", version, err)
		writeError(w, err.Error(), 404)
		return
	}
	fmt.Printf("[修改器] 找到: %s (%s)\n", pack.FileName, pack.Author)
	writeJSON(w, pack)
}

func handleInstallModifier(w http.ResponseWriter, r *http.Request) {
	var req installModifierReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "解析请求失败: "+err.Error(), 400)
		return
	}
	fmt.Printf("[安装修改器] gamePath=%q zip=%q\n", req.GamePath, req.Pack.SourcePath)
	if err := InstallModifier(req.Pack, req.GamePath); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleOnlineVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := FetchVersions(appCfg.ServerURL)
	if err != nil {
		fmt.Printf("[在线] 获取版本失败: %v\n", err)
		writeError(w, err.Error(), 500)
		return
	}
	fmt.Printf("[在线] 版本: %v\n", versions)
	writeJSON(w, versions)
}

func handleOnlineAuthors(w http.ResponseWriter, r *http.Request) {
	authors, err := FetchAuthors(appCfg.ServerURL)
	if err != nil {
		fmt.Printf("[在线] 获取作者失败: %v\n", err)
		writeError(w, err.Error(), 500)
		return
	}
	fmt.Printf("[在线] 作者: %d 位\n", len(authors))
	writeJSON(w, authors)
}

func handleOnlineMods(w http.ResponseWriter, r *http.Request) {
	ver := r.URL.Query().Get("ver")
	author := r.URL.Query().Get("author")
	modType := r.URL.Query().Get("type")
	mods, err := FetchMods(appCfg.ServerURL, ver, author, modType)
	if err != nil {
		fmt.Printf("[在线] 获取MOD失败: %v\n", err)
		writeError(w, err.Error(), 500)
		return
	}
	fmt.Printf("[在线] MOD: ver=%q author=%q type=%q → %d 个\n", ver, author, modType, len(mods))
	writeJSON(w, mods)
}

func handleOnlineInstall(w http.ResponseWriter, r *http.Request) {
	var req installOnlineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "解析请求失败: "+err.Error(), 400)
		return
	}
	fmt.Printf("[在线安装] gamePath=%q name=%q\n", req.GamePath, req.Info.NameCN)
	if err := DownloadAndInstallServerMod(req.Info, req.GamePath, appCfg.ServerURL); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleOpenDir(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		writeError(w, "缺少 path 参数", 400)
		return
	}
	cmd := exec.Command("explorer", dirPath)
	cmd.Start()
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleSelectFolder(w http.ResponseWriter, r *http.Request) {
	path, err := SelectFolder()
	if err != nil {
		writeError(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]string{"path": path})
}
