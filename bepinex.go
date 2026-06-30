package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckBepInEx 检测游戏目录是否已安装 BepInEx
// 对照 gamefile.json 的 BepInEx 列表逐条检查
func CheckBepInEx(gamePath string) (bool, error) {
	manifest, err := LoadGameFileManifest()
	if err != nil {
		return false, err
	}

	if len(manifest.BepInEx) == 0 {
		return false, nil
	}

	for _, entry := range manifest.BepInEx {
		checkPath := filepath.Join(gamePath, CleanDirName(entry))
		if !PathExists(checkPath) {
			return false, nil
		}
	}

	return true, nil
}

// InstallBepInEx 安装 BepInEx 到游戏目录
// 在所有搜索路径中查找 BepInEx.zip，解压到游戏目录
func InstallBepInEx(gamePath string, modPaths []string) error {
	zipPath, err := FindBepInExZip(modPaths)
	if err != nil {
		return err
	}

	fmt.Printf("找到 BepInEx.zip: %s\n", zipPath)
	fmt.Printf("解压到: %s\n", gamePath)

	if err := Unzip(zipPath, gamePath); err != nil {
		return fmt.Errorf("安装 BepInEx 失败: %w", err)
	}

	return nil
}

// UninstallBepInEx 卸载 BepInEx
// 保留 GameFile 白名单内容，删除 BepInEx 清单内容
func UninstallBepInEx(gamePath string) error {
	manifest, err := LoadGameFileManifest()
	if err != nil {
		return err
	}

	fmt.Printf("卸载 BepInEx，游戏目录: %s\n", gamePath)
	fmt.Printf("保留 GameFile: %d 项\n", len(manifest.GameFile))

	// 删除 BepInEx 清单中的每条
	for _, entry := range manifest.BepInEx {
		removePath := filepath.Join(gamePath, CleanDirName(entry))
		if !PathExists(removePath) {
			fmt.Printf("  跳过(不存在): %s\n", entry)
			continue
		}
		fmt.Printf("  删除: %s\n", entry)
		if err := RemovePath(removePath); err != nil {
			return fmt.Errorf("删除 %s 失败: %w", entry, err)
		}
	}

	return nil
}

// FindBepInExZip 在所有搜索路径中查找 BepInEx.zip
// 搜索顺序：mod_paths → 程序执行目录
func FindBepInExZip(searchPaths []string) (string, error) {
	// 1. 在 mod_path 中搜索（包括子目录，如作者目录根）
	for _, modPath := range searchPaths {
		if !DirExists(modPath) {
			continue
		}

		// 先查 mod_path 根目录
		candidate := filepath.Join(modPath, "BepInEx.zip")
		if FileExists(candidate) {
			return candidate, nil
		}

		// 再查 mod_path 下的作者子目录
		authorDirs, err := os.ReadDir(modPath)
		if err != nil {
			continue
		}
		for _, authorDir := range authorDirs {
			if !authorDir.IsDir() {
				continue
			}
			candidate = filepath.Join(modPath, authorDir.Name(), "BepInEx.zip")
			if FileExists(candidate) {
				return candidate, nil
			}
		}
	}

	// 2. 程序执行目录
	exeCandidate := filepath.Join(exeDir(), "BepInEx.zip")
	if FileExists(exeCandidate) {
		return exeCandidate, nil
	}

	return "", fmt.Errorf("未找到 BepInEx.zip（已在 MOD 目录和程序目录中搜索）")
}
