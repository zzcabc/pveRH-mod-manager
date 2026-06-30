package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ===== MOD 模型 =====

// LocalModItem 本地 MOD 条目
type LocalModItem struct {
	DirName     string   `json:"dir_name"`     // 目录原始名
	DisplayName string   `json:"display_name"` // 显示中文名
	Author      string   `json:"author"`
	Version     string   `json:"version"`
	Category    string   `json:"category"` // 植物MOD / 僵尸MOD / ...
	SourcePath  string   `json:"source_path"`
	DllNames    []string `json:"dll_names"` // MOD 包含的 dll 文件名
}

// ===== MOD 类别 =====

var modCategories = []string{"植物MOD", "僵尸MOD", "皮肤MOD", "关卡", "其他"}

// ===== 本地 MOD 扫描 =====

// ScanLocalMods 扫描 MOD 目录中匹配版本的 MOD
// 返回按类别分组的映射
func ScanLocalMods(modPath string, version string) (map[string][]LocalModItem, error) {
	result := make(map[string][]LocalModItem)
	for _, cat := range modCategories {
		result[cat] = []LocalModItem{}
	}

	if !DirExists(modPath) {
		return result, nil
	}

	authorDirs, err := os.ReadDir(modPath)
	if err != nil {
		return nil, fmt.Errorf("读取 MOD 目录失败: %w", err)
	}

	for _, authorDir := range authorDirs {
		if !authorDir.IsDir() {
			continue
		}
		author := authorDir.Name()
		authorPath := filepath.Join(modPath, author)

		verDirs, err := os.ReadDir(authorPath)
		if err != nil {
			continue
		}

		for _, verDir := range verDirs {
			if !verDir.IsDir() {
				continue
			}

			if !MatchVersionDir(verDir.Name(), version) {
				continue
			}

			verPath := filepath.Join(authorPath, verDir.Name())
			extractedVer := ExtractVersion(verDir.Name())

			// 遍历类别目录
			for _, cat := range modCategories {
				catPath := filepath.Join(verPath, cat)
				if !DirExists(catPath) {
					continue
				}

				modDirs, err := os.ReadDir(catPath)
				if err != nil {
					continue
				}

				for _, modDir := range modDirs {
					if !modDir.IsDir() {
						continue
					}

					modPath := filepath.Join(catPath, modDir.Name())
					item := LocalModItem{
						DirName:     modDir.Name(),
						DisplayName: ParseDisplayName(modDir.Name()),
						Author:      author,
						Version:     extractedVer,
						Category:    cat,
						SourcePath:  modPath,
						DllNames:    GetModDllNames(modPath),
					}
					result[cat] = append(result[cat], item)
				}
			}
		}
	}

	// 排序每类列表
	for _, items := range result {
		sort.Slice(items, func(i, j int) bool {
			if items[i].Author != items[j].Author {
				return items[i].Author < items[j].Author
			}
			return items[i].DisplayName < items[j].DisplayName
		})
	}

	return result, nil
}

// ScanInstalledMods 扫描游戏目录 BepInEx/plugins 中已安装的 MOD
// 返回按类别分组的映射（类别通过子目录名推断）
func ScanInstalledMods(gamePath string) (map[string][]LocalModItem, error) {
	result := make(map[string][]LocalModItem)
	for _, cat := range modCategories {
		result[cat] = []LocalModItem{}
	}

	pluginsPath := filepath.Join(gamePath, "BepInEx", "plugins")
	if !DirExists(pluginsPath) {
		return result, nil
	}

	entries, err := os.ReadDir(pluginsPath)
	if err != nil {
		return nil, fmt.Errorf("读取 plugins 目录失败: %w", err)
	}

	// 已知的框架级 DLL（非 MOD），跳过
	frameworkDlls := map[string]bool{
		"customizelib.bepinex.dll":  true,
		"il2cppinterop.common.dll":  true,
		"il2cppinterop.runtime.dll": true,
		"hengmingmodslib.dll":       true,
	}

	for _, entry := range entries {
		var modName string
		var modPath string

		if entry.IsDir() {
			modName = entry.Name()
			modPath = filepath.Join(pluginsPath, modName)
		} else {
			// 单独的 dll 文件也视为一个 MOD
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".dll") {
				continue
			}
			if frameworkDlls[strings.ToLower(name)] {
				continue
			}
			modName = strings.TrimSuffix(name, ".dll")
			modName = strings.TrimSuffix(modName, ".DLL")
			modPath = filepath.Join(pluginsPath, name)
		}

		category := guessCategory(modName)

		item := LocalModItem{
			DirName:     modName,
			DisplayName: ParseDisplayName(modName),
			Author:      "",
			Version:     "",
			Category:    category,
			SourcePath:  modPath,
		}

		result[category] = append(result[category], item)
	}

	return result, nil
}

// ===== MOD 操作 =====

// InstallLocalMod 安装本地 MOD 到游戏目录
// 复制 .dll 文件直接到 BepInEx/plugins/（BepInEx 加载要求）
func InstallLocalMod(item LocalModItem, gamePath string) error {
	gameBepInEx := filepath.Join(gamePath, "BepInEx")
	gamePlugins := filepath.Join(gameBepInEx, "plugins")
	if err := os.MkdirAll(gamePlugins, 0755); err != nil {
		return fmt.Errorf("创建 plugins 目录失败: %w", err)
	}

	modBepInEx := filepath.Join(item.SourcePath, "BepInEx")
	if DirExists(modBepInEx) {
		// 复制 core/ 文件到游戏 BepInEx/core/
		modCore := filepath.Join(modBepInEx, "core")
		if DirExists(modCore) {
			fmt.Printf("  复制 core: %s\n", item.DisplayName)
			if err := CopyDir(modCore, filepath.Join(gameBepInEx, "core")); err != nil {
				return fmt.Errorf("复制 core 失败: %w", err)
			}
		}
		// 复制 plugins/*.dll 直接到游戏 plugins/（不建子目录）
		modPlugins := filepath.Join(modBepInEx, "plugins")
		if DirExists(modPlugins) {
			entries, err := os.ReadDir(modPlugins)
			if err != nil {
				return fmt.Errorf("读取 MOD plugins 失败: %w", err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				src := filepath.Join(modPlugins, e.Name())
				dst := filepath.Join(gamePlugins, e.Name())
				fmt.Printf("  复制: %s → plugins/%s\n", item.DisplayName, e.Name())
				if err := copyFile(src, dst); err != nil {
					return fmt.Errorf("复制 %s 失败: %w", e.Name(), err)
				}
			}
		}
		return nil
	}

	// 无 BepInEx 结构：复制整个 MOD 目录下所有文件到 plugins/ 子目录或直接放 dll
	entries, err := os.ReadDir(item.SourcePath)
	if err != nil {
		return fmt.Errorf("读取 MOD 目录失败: %w", err)
	}
	hasDll := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			hasDll = true
			src := filepath.Join(item.SourcePath, e.Name())
			dst := filepath.Join(gamePlugins, e.Name())
			fmt.Printf("  复制: %s → plugins/%s\n", item.DisplayName, e.Name())
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("复制 %s 失败: %w", e.Name(), err)
			}
		}
	}
	if !hasDll {
		// 无 dll 文件，整目录复制
		dst := filepath.Join(gamePlugins, item.DirName)
		fmt.Printf("  复制目录: %s → %s\n", item.DisplayName, dst)
		return CopyDir(item.SourcePath, dst)
	}
	return nil
}

// GetModDllNames 获取 MOD 包含的 dll 文件名列表（用于安装状态检测）
func GetModDllNames(sourcePath string) []string {
	var names []string
	modBepInEx := filepath.Join(sourcePath, "BepInEx")
	searchDir := modBepInEx
	if !DirExists(modBepInEx) {
		searchDir = sourcePath
	}

	// 查找 plugins/ 子目录下的 dll
	if pluginsDir := filepath.Join(searchDir, "plugins"); DirExists(pluginsDir) {
		entries, _ := os.ReadDir(pluginsDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
				names = append(names, e.Name())
			}
		}
	} else {
		// 直接在目录下找 dll
		entries, _ := os.ReadDir(searchDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
				names = append(names, e.Name())
			}
		}
	}
	return names
}

// IsModInstalled 判断 MOD 是否已安装（通过检测其 dll 文件是否存在）
func IsModInstalled(sourcePath, gamePath string) bool {
	dlls := GetModDllNames(sourcePath)
	if len(dlls) == 0 {
		return false
	}
	pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")
	for _, dll := range dlls {
		if FileExists(filepath.Join(pluginsDir, dll)) {
			return true
		}
	}
	return false
}

// UninstallMod 卸载单个 MOD（直接删除 SourcePath 指向的目录）
func UninstallMod(item LocalModItem) error {
	if !PathExists(item.SourcePath) {
		return fmt.Errorf("MOD 路径不存在: %s", item.SourcePath)
	}

	fmt.Printf("卸载 MOD: %s (%s)\n", item.DisplayName, item.SourcePath)
	return RemovePath(item.SourcePath)
}

// UninstallAllMods 卸载游戏目录下的全部 MOD（清空 BepInEx/plugins）
func UninstallAllMods(gamePath string) error {
	pluginsPath := filepath.Join(gamePath, "BepInEx", "plugins")
	if !DirExists(pluginsPath) {
		return nil
	}

	fmt.Printf("清空全部 MOD: %s\n", pluginsPath)

	entries, err := os.ReadDir(pluginsPath)
	if err != nil {
		return fmt.Errorf("读取 plugins 目录失败: %w", err)
	}

	for _, entry := range entries {
		itemPath := filepath.Join(pluginsPath, entry.Name())
		if err := RemovePath(itemPath); err != nil {
			return fmt.Errorf("删除 %s 失败: %w", entry.Name(), err)
		}
	}

	return nil
}

// ===== 辅助 =====

// guessCategory 根据目录名猜测 MOD 类别
func guessCategory(dirName string) string {
	dirNameLower := filepath.Base(dirName)

	// 包含特定关键词则归类
	catKeywords := map[string]string{
		"僵尸":     "僵尸MOD",
		"zombie": "僵尸MOD",
		"皮肤":     "皮肤MOD",
		"skin":   "皮肤MOD",
		"关卡":     "关卡",
		"level":  "关卡",
		"植物":     "植物MOD",
		"plant":  "植物MOD",
	}

	for keyword, cat := range catKeywords {
		if contains(dirNameLower, keyword) {
			return cat
		}
	}

	return "植物MOD" // 默认归类植物MOD
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
