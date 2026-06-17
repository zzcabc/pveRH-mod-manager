package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// AvailableMod 代表 Mod 库中的一个可用 Mod（文件夹或 ZIP）
type AvailableMod struct {
	Name     string // 中文译名（文件夹相对路径，用 - 连接）
	IsZip    bool
	ZipPath  string
	DirPath  string
	DllNames []string // 主 dll 文件名列表
}

// 需要跳过的文件夹名称（不区分大小写）
var excludeFolders = []string{"6.bepinex前置框架", "core", "dotnet"}

func isExcluded(name string) bool {
	for _, ex := range excludeFolders {
		if strings.EqualFold(name, ex) {
			return true
		}
	}
	return false
}

// checkModFolder 判断目录是否为有效 Mod 文件夹，返回所有 dll 文件名（排除 CustomizeLib.BepInEx.dll）
func checkModFolder(dir string) (bool, []string) {
	var dlls []string
	collectDlls := func(entries []os.DirEntry) {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
				if strings.EqualFold(e.Name(), "CustomizeLib.BepInEx.dll") {
					continue
				}
				dlls = append(dlls, e.Name())
			}
		}
	}

	// 根目录
	entries, err := os.ReadDir(dir)
	if err == nil {
		collectDlls(entries)
	}

	// plugins 或 BepInEx/plugins 子目录
	for _, sub := range []string{"plugins", filepath.Join("BepInEx", "plugins")} {
		subDir := filepath.Join(dir, sub)
		if info, err := os.Stat(subDir); err == nil && info.IsDir() {
			subEntries, _ := os.ReadDir(subDir)
			collectDlls(subEntries)
		}
	}

	return len(dlls) > 0, dlls
}

// ScanModLibrary 扫描 Mod 库，返回所有 ZIP 和文件夹 Mod
func ScanModLibrary(modLibPath string) ([]AvailableMod, error) {
	logger.Info("正在扫描 Mod 库...")
	var mods []AvailableMod
	seen := make(map[string]bool)       // ZIP 去重
	seenFolder := make(map[string]bool) // 文件夹去重

	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == modLibPath {
			return nil
		}
		if info.IsDir() && isExcluded(info.Name()) {
			return filepath.SkipDir
		}

		// ZIP 处理（跳过已有同名文件夹的）
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".zip") &&
			!strings.EqualFold(info.Name(), "BepInEx.zip") {
			name := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			if seen[name] {
				return nil
			}
			// 检查同目录下是否已有同名文件夹
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), name)); err == nil {
				return nil
			}
			seen[name] = true
			mods = append(mods, AvailableMod{
				Name:    name,
				IsZip:   true,
				ZipPath: path,
			})
			return nil
		}

		// 文件夹处理（多级路径作为中文名）
		if info.IsDir() {
			ok, dllNames := checkModFolder(path)
			if ok {
				rel, err := filepath.Rel(modLibPath, path)
				if err != nil {
					return err
				}
				name := strings.ReplaceAll(rel, string(os.PathSeparator), "-")
				if seenFolder[name] {
					return filepath.SkipDir
				}
				seenFolder[name] = true
				mods = append(mods, AvailableMod{
					Name:     name,
					IsZip:    false,
					DirPath:  path,
					DllNames: dllNames,
				})
				return filepath.SkipDir // 阻止深入子目录
			}
		}
		return nil
	})
	if err != nil {
		logger.Errorf("扫描 Mod 库失败: %v", err)
		return mods, err
	}
	zipCount := 0
	for _, m := range mods {
		if m.IsZip {
			zipCount++
		}
	}
	logger.Infof("Mod 库扫描完成: 共 %d 个 Mod (ZIP: %d, 文件夹: %d)", len(mods), zipCount, len(mods)-zipCount)
	return mods, err
}

// GetInstalledMods 返回已安装的 dll 文件名（含 .dll）
func GetInstalledMods(gamePath string) ([]string, error) {
	pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")
	logger.Debugf("获取已安装 Mod: %s", pluginsDir)
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}
	var mods []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			mods = append(mods, e.Name())
		}
	}
	logger.Debugf("已安装 %d 个 Mod dll", len(mods))
	return mods, nil
}

// FindCustomizeLibDll 在 Mod 库中递归查找 CustomizeLib.BepInEx.dll
func FindCustomizeLibDll(modLibPath string) (string, error) {
	logger.Debugf("搜索 CustomizeLib.BepInEx.dll: %s", modLibPath)
	var found string
	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if strings.EqualFold(info.Name(), "CustomizeLib.BepInEx.dll") {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		logger.Warn("CustomizeLib.BepInEx.dll 未在 Mod 库中找到")
		return "", fmt.Errorf("在 Mod 库中未找到 CustomizeLib.BepInEx.dll")
	}
	logger.Debugf("找到 CustomizeLib.BepInEx.dll: %s", found)
	return found, nil
}

// InstallMod 安装 Mod（复制所有 dll 到 plugins 根目录，自动补充 CustomizeLib.BepInEx.dll）
func InstallMod(mod AvailableMod, gamePath, modLibPath string) error {
	logger.Infof("安装 Mod: %s", mod.Name)
	pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")

	// 依赖检查：CustomizeLib.BepInEx.dll
	customLibPath := filepath.Join(pluginsDir, "CustomizeLib.BepInEx.dll")
	if _, err := os.Stat(customLibPath); os.IsNotExist(err) {
		logger.Debug("复制依赖 CustomizeLib.BepInEx.dll 到 plugins")
		depSrc, err := FindCustomizeLibDll(modLibPath)
		if err != nil {
			logger.Errorf("缺少依赖 CustomizeLib.BepInEx.dll: %v", err)
			return fmt.Errorf("缺少依赖 CustomizeLib.BepInEx.dll 且在 Mod 库中未找到: %v", err)
		}
		depData, err := os.ReadFile(depSrc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(customLibPath, depData, 0644); err != nil {
			return err
		}
	}

	srcDir := mod.DirPath
	for _, dllName := range mod.DllNames {
		var srcPath string
		// 搜索优先级：根目录、plugins、BepInEx/plugins
		for _, sub := range []string{srcDir, filepath.Join(srcDir, "plugins"), filepath.Join(srcDir, "BepInEx", "plugins")} {
			p := filepath.Join(sub, dllName)
			if _, err := os.Stat(p); err == nil {
				srcPath = p
				break
			}
		}
		if srcPath == "" {
			logger.Errorf("未找到 dll 文件: %s (Mod: %s)", dllName, mod.Name)
			return fmt.Errorf("未找到 dll 文件: %s", dllName)
		}
		destPath := filepath.Join(pluginsDir, dllName)
		logger.Debugf("复制 dll: %s -> %s", filepath.Base(srcPath), destPath)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}
	logger.Infof("Mod 安装完成: %s", mod.Name)
	return nil
}

// UninstallMod 卸载 Mod（根据中文名删除所有对应 dll）
func UninstallMod(chineseName, gamePath string, mods []AvailableMod) error {
	// 先尝试从已知 Mod 中查找对应 dll 列表
	logger.Infof("卸载 Mod: %s", chineseName)
	var targetDlls []string
	for _, m := range mods {
		if m.Name == chineseName && !m.IsZip {
			targetDlls = m.DllNames
			break
		}
	}

	if len(targetDlls) > 0 {
		pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")
		for _, dll := range targetDlls {
			p := filepath.Join(pluginsDir, dll)
			if _, err := os.Stat(p); err == nil {
				logger.Debugf("删除 dll: %s", p)
				os.Remove(p)
			}
		}
		return nil
	}

	// 未知 Mod：直接作为 dll 文件名删除
	dllName := chineseName
	if !strings.HasSuffix(strings.ToLower(dllName), ".dll") {
		dllName += ".dll"
	}
	dllPath := filepath.Join(gamePath, "BepInEx", "plugins", dllName)
	if _, err := os.Stat(dllPath); os.IsNotExist(err) {
		return fmt.Errorf("未找到 Mod 或 dll: %s", chineseName)
	}
	logger.Infof("Mod 卸载完成: %s", chineseName)
	return os.Remove(dllPath)
}

// IsZombieMod 判断是否为僵尸 Mod
func IsZombieMod(name string, dllNames []string) bool {
	if strings.Contains(strings.ToLower(name), "僵尸") || strings.Contains(strings.ToLower(name), "zombie") {
		return true
	}
	for _, dll := range dllNames {
		if strings.Contains(strings.ToLower(dll), "僵尸") || strings.Contains(strings.ToLower(dll), "zombie") {
			return true
		}
	}
	return false
}

// NeedsFormat 判断 Mod 文件夹是否需要格式化
func NeedsFormat(dir string) bool {
	for _, sub := range []string{"plugins", filepath.Join("BepInEx", "plugins")} {
		if info, err := os.Stat(filepath.Join(dir, sub)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// FormatModFolder 格式化 Mod 文件夹（移动 dll 到根目录，删除多余目录）
func FormatModFolder(modName, modLibPath string) error {
	logger.Infof("格式化 Mod 文件夹: %s", modName)
	mods, _ := ScanModLibrary(modLibPath)
	var dirPath string
	var dllNames []string
	for _, m := range mods {
		if m.Name == modName && !m.IsZip {
			dirPath = m.DirPath
			dllNames = m.DllNames
			break
		}
	}
	if dirPath == "" {
		return fmt.Errorf("未找到 Mod 文件夹: %s", modName)
	}

	// 将所有 dll 移动到根目录
	for _, dll := range dllNames {
		src := ""
		for _, sub := range []string{dirPath, filepath.Join(dirPath, "plugins"), filepath.Join(dirPath, "BepInEx", "plugins")} {
			p := filepath.Join(sub, dll)
			if _, err := os.Stat(p); err == nil {
				src = p
				break
			}
		}
		if src == "" {
			continue
		}
		dst := filepath.Join(dirPath, dll)
		if src != dst {
			logger.Debugf("移动 dll: %s -> %s", src, dst)
			os.Remove(dst) // 覆盖
			if err := os.Rename(src, dst); err != nil {
				logger.Errorf("移动 dll 失败: %s, %v", dll, err)
				return fmt.Errorf("移动 %s 失败: %v", dll, err)
			}
		}
	}

	// 删除多余目录
	for _, folder := range []string{"plugins", "BepInEx", "License"} {
		os.RemoveAll(filepath.Join(dirPath, folder))
	}
	logger.Infof("Mod 格式化完成: %s", modName)
	return nil
}

// UnzipModToDir 解压 ZIP 到同名文件夹（覆盖式）
func UnzipModToDir(modName, modLibPath string) error {
	logger.Infof("解压 Mod ZIP: %s", modName)
	mods, _ := ScanModLibrary(modLibPath)
	var zipPath string
	for _, m := range mods {
		if m.Name == modName && m.IsZip {
			zipPath = m.ZipPath
			break
		}
	}
	if zipPath == "" {
		return fmt.Errorf("未找到 ZIP: %s", modName)
	}
	destDir := filepath.Join(filepath.Dir(zipPath), modName)
	os.RemoveAll(destDir)
	os.MkdirAll(destDir, os.ModePerm)
	logger.Debugf("解压 %s 到 %s", zipPath, destDir)
	if err := Unzip(zipPath, destDir); err != nil {
		logger.Errorf("解压失败: %s, %v", modName, err)
		return err
	}
	logger.Infof("解压完成: %s", modName)
	return nil
}

// ---------- 皮肤相关 ----------

// SkinMod 皮肤 Mod
type SkinMod struct {
	Name    string // 中文名（相对路径，用 - 连接）
	DirPath string // 源目录
}

// isSkinFolder 检查目录是否为皮肤文件夹
func isSkinFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "skin_") {
			return true
		}
		if e.IsDir() {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				subDir := filepath.Join(dir, e.Name())
				subEntries, _ := os.ReadDir(subDir)
				if len(subEntries) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// ScanSkinLibrary 扫描皮肤库
func ScanSkinLibrary(modLibPath string) ([]SkinMod, error) {
	logger.Info("正在扫描皮肤库...")
	var skins []SkinMod
	seen := make(map[string]bool)

	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == modLibPath {
			return nil
		}
		if info.IsDir() && isExcluded(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() && isSkinFolder(path) {
			rel, err := filepath.Rel(modLibPath, path)
			if err != nil {
				return err
			}
			name := strings.ReplaceAll(rel, string(os.PathSeparator), "-")
			if seen[name] {
				return nil
			}
			seen[name] = true
			skins = append(skins, SkinMod{
				Name:    name,
				DirPath: path,
			})
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		logger.Errorf("扫描皮肤库失败: %v", err)
		return skins, err
	}
	logger.Infof("皮肤库扫描完成: 共 %d 个皮肤", len(skins))
	return skins, err
}

// GetInstalledSkins 返回已安装的皮肤名称
func GetInstalledSkins(gamePath string) ([]string, error) {
	skinDir := filepath.Join(gamePath, "BepInEx", "plugins", "skin")
	logger.Debugf("获取已安装皮肤: %s", skinDir)
	entries, err := os.ReadDir(skinDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	logger.Debugf("已安装 %d 个皮肤", len(names))
	return names, nil
}

// FindSkinLoaderDll 在 Mod 库中查找 SkinLoader.dll
func FindSkinLoaderDll(modLibPath string) (string, error) {
	logger.Debugf("搜索 SkinLoader.dll: %s", modLibPath)
	var found string
	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if strings.EqualFold(info.Name(), "SkinLoader.dll") {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		logger.Warn("SkinLoader.dll 未在 Mod 库中找到")
		return "", fmt.Errorf("在 Mod 库中未找到 SkinLoader.dll")
	}
	logger.Debugf("找到 SkinLoader.dll: %s", found)
	return found, nil
}

// InstallSkin 安装皮肤，自动复制 SkinLoader.dll
func InstallSkin(skin SkinMod, gamePath, modLibPath string) error {
	logger.Infof("安装皮肤: %s", skin.Name)
	pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")

	// 检查并复制 SkinLoader.dll
	skinLoaderPath := filepath.Join(pluginsDir, "SkinLoader.dll")
	if _, err := os.Stat(skinLoaderPath); os.IsNotExist(err) {
		logger.Debug("复制 SkinLoader.dll 到 plugins")
		src, err := FindSkinLoaderDll(modLibPath)
		if err != nil {
			logger.Errorf("缺少 SkinLoader.dll: %v", err)
			return fmt.Errorf("缺少 SkinLoader.dll 且在 Mod 库中未找到: %v", err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(skinLoaderPath, data, 0644); err != nil {
			return err
		}
	}

	skinDir := filepath.Join(gamePath, "BepInEx", "plugins", "skin")
	if err := os.MkdirAll(skinDir, os.ModePerm); err != nil {
		return err
	}
	destDir := filepath.Join(skinDir, skin.Name)
	os.RemoveAll(destDir)
	logger.Debugf("复制皮肤目录: %s -> %s", skin.DirPath, destDir)
	if err := CopyDir(skin.DirPath, destDir); err != nil {
		logger.Errorf("皮肤安装失败: %s, %v", skin.Name, err)
		return err
	}
	logger.Infof("皮肤安装完成: %s", skin.Name)
	return nil
}

// UninstallSkin 卸载皮肤
func UninstallSkin(name, gamePath string) error {
	logger.Infof("卸载皮肤: %s", name)
	skinDir := filepath.Join(gamePath, "BepInEx", "plugins", "skin", name)
	if _, err := os.Stat(skinDir); os.IsNotExist(err) {
		return fmt.Errorf("未安装该皮肤: %s", name)
	}
	if err := os.RemoveAll(skinDir); err != nil {
		logger.Errorf("皮肤卸载失败: %s, %v", name, err)
		return err
	}
	logger.Infof("皮肤卸载完成: %s", name)
	return nil
}
