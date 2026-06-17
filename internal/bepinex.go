package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// IsBepInExInstalled 检查游戏目录下是否已安装 BepInEx
func IsBepInExInstalled(gamePath string) bool {
	logger.Debugf("检查 BepInEx 安装状态: %s", gamePath)
	bepDir := filepath.Join(gamePath, "BepInEx")
	info, err := os.Stat(bepDir)
	installed := err == nil && info.IsDir()
	if installed {
		logger.Debug("BepInEx 已安装")
	} else {
		logger.Debug("BepInEx 未安装")
	}
	return installed
}

// InstallBepInEx 在 Mod 库中递归查找 BepInEx.zip 并解压到游戏根目录
func InstallBepInEx(gamePath, modLibPath string) error {
	logger.Infof("开始安装 BepInEx: game=%s, modlib=%s", gamePath, modLibPath)
	zipPath, err := FindBepInExZip(modLibPath)
	if err != nil {
		logger.Errorf("BepInEx 安装失败: 未找到 BepInEx.zip, %v", err)
		return fmt.Errorf("在 Mod 库中未找到 BepInEx.zip: %v", err)
	}
	logger.Debugf("找到 BepInEx.zip: %s", zipPath)
	logger.Infof("正在解压 BepInEx.zip 到 %s", gamePath)
	if err := Unzip(zipPath, gamePath); err != nil {
		logger.Errorf("BepInEx 解压失败: %v", err)
		return err
	}
	logger.Info("BepInEx 安装完成")
	return nil
}

// FindBepInExZip 递归搜索 modLibPath 目录，找到第一个 BepInEx.zip（不区分大小写）
func FindBepInExZip(modLibPath string) (string, error) {
	logger.Debugf("搜索 BepInEx.zip: %s", modLibPath)
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
	logger.Debugf("找到 BepInEx.zip: %s", found)
	return found, nil
}
