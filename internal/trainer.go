package internal

import (
	"os"
	"path/filepath"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// TrainerMod 修改器信息
type TrainerMod struct {
	Name  string // 显示名称（文件名）
	Path  string // 完整路径
	IsZip bool   // true=zip, false=rar
}

// ScanTrainerLibrary 在 Mod 库中递归查找修改器 ZIP/RAR
func ScanTrainerLibrary(modLibPath string) ([]TrainerMod, error) {
	logger.Info("正在扫描修改器库...")
	var trainers []TrainerMod
	seen := make(map[string]bool)

	err := filepath.Walk(modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		lower := strings.ToLower(name)
		// 检查是否为目标修改器（包含关键字）
		isTarget := strings.Contains(lower, "pvzrhmodfied") || strings.Contains(lower, "PvZRHModfied") ||
			strings.Contains(lower, "pvzrhtools") || strings.Contains(lower, "PVZRHTools")
		if !isTarget {
			return nil
		}

		if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".rar") {
			if seen[name] {
				return nil
			}
			seen[name] = true
			trainers = append(trainers, TrainerMod{
				Name:  name,
				Path:  path,
				IsZip: strings.HasSuffix(lower, ".zip"),
			})
		}
		return nil
	})
	if err != nil {
		logger.Errorf("扫描修改器失败: %v", err)
		return trainers, err
	}
	logger.Infof("修改器扫描完成: 共 %d 个", len(trainers))
	return trainers, err
}

// InstallTrainer 安装修改器：根据类型解压到游戏根目录
func InstallTrainer(trainer TrainerMod, gamePath string) error {
	ext := "rar"
	if trainer.IsZip {
		ext = "zip"
	}
	logger.Infof("安装修改器: %s (类型: %s)", trainer.Name, ext)
	var err error
	if trainer.IsZip {
		err = Unzip(trainer.Path, gamePath)
	} else {
		err = Unrar(trainer.Path, gamePath)
	}
	if err != nil {
		logger.Errorf("修改器安装失败: %s, %v", trainer.Name, err)
		return err
	}
	logger.Infof("修改器安装完成: %s", trainer.Name)
	return nil
}
