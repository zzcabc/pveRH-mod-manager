package internal

import (
	"os"
	"path/filepath"
	"strings"
)

// TrainerMod 修改器信息
type TrainerMod struct {
	Name  string // 显示名称（文件名）
	Path  string // 完整路径
	IsZip bool   // true=zip, false=rar
}

// ScanTrainerLibrary 在 Mod 库中递归查找修改器 ZIP/RAR
func ScanTrainerLibrary(modLibPath string) ([]TrainerMod, error) {
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
		isTarget := strings.Contains(lower, "pvzrhmodfied") || strings.Contains(lower, "高数修改器") ||
			strings.Contains(lower, "pvzrhtools") || strings.Contains(lower, "梧萱修改器")
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
	return trainers, err
}

// InstallTrainer 安装修改器：根据类型解压到游戏根目录
func InstallTrainer(trainer TrainerMod, gamePath string) error {
	if trainer.IsZip {
		return Unzip(trainer.Path, gamePath)
	}
	return Unrar(trainer.Path, gamePath)
}
