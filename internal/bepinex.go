package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsBepInExInstalled 检查游戏目录下是否已安装 BepInEx
func IsBepInExInstalled(gamePath string) bool {
	bepDir := filepath.Join(gamePath, "BepInEx")
	info, err := os.Stat(bepDir)
	return err == nil && info.IsDir()
}

// InstallBepInEx 在 Mod 库中递归查找 BepInEx.zip 并解压到游戏根目录
func InstallBepInEx(gamePath, modLibPath string) error {
	zipPath, err := FindBepInExZip(modLibPath)
	if err != nil {
		return fmt.Errorf("在 Mod 库中未找到 BepInEx.zip: %v", err)
	}
	return Unzip(zipPath, gamePath)
}

// FindBepInExZip 递归搜索 modLibPath 目录，找到第一个 BepInEx.zip（不区分大小写）
func FindBepInExZip(modLibPath string) (string, error) {
	var found string
	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(info.Name(), "BepInEx.zip") {
			found = path
			return filepath.SkipAll // 找到后立即停止遍历
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("未找到 BepInEx.zip")
	}
	return found, nil
}
