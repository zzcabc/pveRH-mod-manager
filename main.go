package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/browser"
	"github.com/sqweek/dialog"

	"pveRH-mod-manager/internal"
)

//go:embed web
var webFS embed.FS

var (
	gamePath   string
	modLibPath string
)

const onlineModServer = "https://pvzrhmod.zhaocheng.cc:8443" // 在线 Mod 服务器地址

// Config 持久化配置
type Config struct {
	GamePath   string `json:"game_path"`
	ModLibPath string `json:"modlib_path"`
}

func configFilePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exePath), "config.json")
}

func loadConfig() {
	data, err := os.ReadFile(configFilePath())
	log.Printf("正在加载配置文件: %s", data)
	if err != nil {
		return
	}
	var cfg Config
	if json.Unmarshal(data, &cfg) == nil {
		gamePath = cfg.GamePath
		modLibPath = cfg.ModLibPath
	}
}

func saveConfig() {
	cfg := Config{
		GamePath:   gamePath,
		ModLibPath: modLibPath,
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configFilePath(), data, 0644)
}

func main() {
	loadConfig()

	r := mux.NewRouter()

	// API 路由
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/select-game", handleSelectGame).Methods("GET")
	api.HandleFunc("/select-modlib", handleSelectModLib).Methods("GET")
	api.HandleFunc("/check-bepinex", handleCheckBepInEx).Methods("GET")
	api.HandleFunc("/install-bepinex", handleInstallBepInEx).Methods("POST")
	api.HandleFunc("/mods", handleGetMods).Methods("GET")
	api.HandleFunc("/install-mod", handleInstallMod).Methods("POST")
	api.HandleFunc("/uninstall-mod", handleUninstallMod).Methods("POST")
	api.HandleFunc("/unzip-mod", handleUnzipMod).Methods("POST")
	api.HandleFunc("/format-mod", handleFormatMod).Methods("POST")
	api.HandleFunc("/format-all", handleFormatAll).Methods("POST")
	api.HandleFunc("/unzip-all", handleUnzipAll).Methods("POST")
	api.HandleFunc("/skins", handleGetSkins).Methods("GET")
	api.HandleFunc("/install-skin", handleInstallSkin).Methods("POST")
	api.HandleFunc("/uninstall-skin", handleUninstallSkin).Methods("POST")
	api.HandleFunc("/trainers", handleGetTrainers).Methods("GET")
	api.HandleFunc("/install-trainer", handleInstallTrainer).Methods("POST")
	api.HandleFunc("/game-versions", handleGameVersions).Methods("GET")
	api.HandleFunc("/online-mods", handleOnlineMods).Methods("GET")
	api.HandleFunc("/download-mod", handleDownloadMod).Methods("POST")
	api.HandleFunc("/config", handleGetConfig).Methods("GET")

	// 静态文件服务
	webDir, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal("嵌入 web 目录失败: ", err)
	}
	r.PathPrefix("/").Handler(http.FileServer(http.FS(webDir)))

	// 自动打开浏览器
	go func() {
		browser.OpenURL("http://localhost:8080")
	}()

	log.Println("服务已启动：http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// ---------- 文件夹选择 ----------
func handleSelectGame(w http.ResponseWriter, r *http.Request) {
	path, err := selectFolder("选择游戏根目录")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"path": ""})
		return
	}
	gamePath = path
	saveConfig()
	json.NewEncoder(w).Encode(map[string]string{"path": gamePath})
}

func handleSelectModLib(w http.ResponseWriter, r *http.Request) {
	path, err := selectFolder("选择 Mod 库文件夹")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"path": ""})
		return
	}
	modLibPath = path
	saveConfig()
	json.NewEncoder(w).Encode(map[string]string{"path": modLibPath})
}

func selectFolder(title string) (string, error) {
	return dialog.Directory().Title(title).Browse()
}

// ---------- BepInEx ----------
func handleCheckBepInEx(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" {
		http.Error(w, "请先选择游戏目录", http.StatusBadRequest)
		return
	}
	installed := internal.IsBepInExInstalled(gamePath)
	log.Printf("正在检查 BepInEx 是否已安装: %t", installed)
	json.NewEncoder(w).Encode(map[string]bool{"installed": installed})
}

func handleInstallBepInEx(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "目录未设置", http.StatusBadRequest)
		return
	}
	err := internal.InstallBepInEx(gamePath, modLibPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("正在安装 BepInEx 到 %s", gamePath)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- Mod 列表 ----------
func handleGetMods(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "请先设置目录", http.StatusBadRequest)
		return
	}

	installedDlls, _ := internal.GetInstalledMods(gamePath)
	available, _ := internal.ScanModLibrary(modLibPath)

	// 构建 dll 小写 → 中文名，以及中文名 → DllNames 映射
	dllToChinese := make(map[string]string)
	modDllMap := make(map[string][]string)
	for _, m := range available {
		if m.IsZip {
			continue
		}
		modDllMap[m.Name] = m.DllNames
		for _, dll := range m.DllNames {
			dllToChinese[strings.ToLower(dll)] = m.Name
		}
	}

	// 已安装：按 mod 聚合
	installedModMap := make(map[string][]string) // 中文名 → dll列表
	for _, dll := range installedDlls {
		lower := strings.ToLower(dll)
		cnName, ok := dllToChinese[lower]
		if !ok {
			cnName = dll
		}
		installedModMap[cnName] = append(installedModMap[cnName], dll)
	}

	installedSet := make(map[string]bool)
	var installedPlant, installedZombie []map[string]interface{}

	for cnName, dlls := range installedModMap {
		installedSet[cnName] = true
		entry := map[string]interface{}{
			"chinese_name": cnName,
			"dll_names":    dlls,
		}
		if internal.IsZombieMod(cnName, dlls) {
			installedZombie = append(installedZombie, entry)
		} else {
			installedPlant = append(installedPlant, entry)
		}
	}

	// 未安装
	var notInstalledPlant, notInstalledZombie []map[string]interface{}
	for _, m := range available {
		if m.IsZip {
			continue
		}
		if !installedSet[m.Name] {
			entry := map[string]interface{}{
				"name":         m.Name,
				"dll_names":    m.DllNames,
				"needs_format": internal.NeedsFormat(m.DirPath),
			}
			if internal.IsZombieMod(m.Name, m.DllNames) {
				notInstalledZombie = append(notInstalledZombie, entry)
			} else {
				notInstalledPlant = append(notInstalledPlant, entry)
			}
		}
	}

	// ZIP 列表
	var zips []map[string]interface{}
	for _, m := range available {
		if m.IsZip {
			zips = append(zips, map[string]interface{}{
				"name": m.Name,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"installed_plant":      installedPlant,
		"installed_zombie":     installedZombie,
		"not_installed_plant":  notInstalledPlant,
		"not_installed_zombie": notInstalledZombie,
		"zips":                 zips,
	})
}

// ---------- 安装/卸载 Mod ----------
func handleInstallMod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

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

	err := internal.InstallMod(target, gamePath, modLibPath)
	log.Printf("正在安装 Mod %s 到 %s", target.Name, gamePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleUninstallMod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"` // 中文译名
	}
	json.NewDecoder(r.Body).Decode(&req)

	mods, _ := internal.ScanModLibrary(modLibPath)
	err := internal.UninstallMod(req.Name, gamePath, mods)
	log.Printf("正在卸载 Mod %s 从 %s", req.Name, gamePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 解压 ZIP ----------
func handleUnzipMod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := internal.UnzipModToDir(req.Name, modLibPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 格式化 ----------
func handleFormatMod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := internal.FormatModFolder(req.Name, modLibPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleFormatAll(w http.ResponseWriter, r *http.Request) {
	if modLibPath == "" {
		http.Error(w, "请先选择 Mod 库", http.StatusBadRequest)
		return
	}
	available, _ := internal.ScanModLibrary(modLibPath)
	var errors []string
	for _, m := range available {
		if m.IsZip {
			continue
		}
		if internal.NeedsFormat(m.DirPath) {
			if err := internal.FormatModFolder(m.Name, modLibPath); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", m.Name, err))
			}
		}
	}
	if len(errors) > 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("部分格式化失败: %s", strings.Join(errors, "; ")),
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleUnzipAll(w http.ResponseWriter, r *http.Request) {
	if modLibPath == "" {
		http.Error(w, "请先选择 Mod 库", http.StatusBadRequest)
		return
	}
	available, _ := internal.ScanModLibrary(modLibPath)
	var errors []string
	for _, m := range available {
		if m.IsZip {
			if err := internal.UnzipModToDir(m.Name, modLibPath); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", m.Name, err))
			}
		}
	}
	if len(errors) > 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("部分解压失败: %s", strings.Join(errors, "; ")),
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 皮肤 ----------
func handleGetSkins(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "请先设置目录", http.StatusBadRequest)
		return
	}
	installed, _ := internal.GetInstalledSkins(gamePath)
	available, _ := internal.ScanSkinLibrary(modLibPath)

	installedSet := make(map[string]bool)
	for _, name := range installed {
		installedSet[name] = true
	}
	var notInstalled []string
	for _, sk := range available {
		if !installedSet[sk.Name] {
			notInstalled = append(notInstalled, sk.Name)
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"installed":     installed,
		"not_installed": notInstalled,
	})
}

func handleInstallSkin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	skins, _ := internal.ScanSkinLibrary(modLibPath)
	var target internal.SkinMod
	for _, sk := range skins {
		if sk.Name == req.Name {
			target = sk
			break
		}
	}
	if target.Name == "" {
		http.Error(w, "皮肤未找到", http.StatusNotFound)
		return
	}
	if err := internal.InstallSkin(target, gamePath, modLibPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleUninstallSkin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := internal.UninstallSkin(req.Name, gamePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 修改器 ----------
func handleGetTrainers(w http.ResponseWriter, r *http.Request) {
	if modLibPath == "" {
		http.Error(w, "请先选择 Mod 库", http.StatusBadRequest)
		return
	}
	trainers, err := internal.ScanTrainerLibrary(modLibPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trainers": trainers,
	})
}

func handleInstallTrainer(w http.ResponseWriter, r *http.Request) {
	if gamePath == "" || modLibPath == "" {
		http.Error(w, "请先选择游戏目录和 Mod 库", http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	trainers, _ := internal.ScanTrainerLibrary(modLibPath)
	var target internal.TrainerMod
	for _, t := range trainers {
		if t.Name == req.Name {
			target = t
			break
		}
	}
	if target.Name == "" {
		http.Error(w, "修改器未找到", http.StatusNotFound)
		return
	}
	if err := internal.InstallTrainer(target, gamePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 在线 Mod ----------
func handleGameVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := internal.FetchGameVersions(onlineModServer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
	})
}

func handleOnlineMods(w http.ResponseWriter, r *http.Request) {
	gameVer := r.URL.Query().Get("ver")
	mods, err := internal.FetchOnlineMods(onlineModServer, gameVer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mods": mods,
	})
}

func handleDownloadMod(w http.ResponseWriter, r *http.Request) {
	if modLibPath == "" {
		http.Error(w, "请先选择 Mod 库", http.StatusBadRequest)
		return
	}
	var mod internal.OnlineMod
	if err := json.NewDecoder(r.Body).Decode(&mod); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}
	log.Printf("下载 Mod: %s", mod)
	if err := internal.DownloadMod(mod, modLibPath, onlineModServer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------- 配置 ----------
func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"game_path":   gamePath,
		"modlib_path": modLibPath,
	})
}
