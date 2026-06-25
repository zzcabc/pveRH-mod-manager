package internal

import (
	"encoding/json"
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

// GameFileConfig 游戏文件配置
type GameFileConfig struct {
	GameFile []string `json:"GameFile"`
	BepInEx  []string `json:"BepInEx"`
}

// LoadGameFileConfigFromData 从嵌入的数据加载 gamefile.json 配置
func LoadGameFileConfigFromData(data []byte) (*GameFileConfig, error) {
	var config GameFileConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 gamefile.json 失败: %v", err)
	}
	return &config, nil
}

// 全局变量，由 main 包初始化
var embeddedGameFileData []byte

// SetEmbeddedGameFileData 设置嵌入的 gamefile.json 数据
func SetEmbeddedGameFileData(data []byte) {
	embeddedGameFileData = data
}

// LoadGameFileConfig 加载 gamefile.json 配置
func LoadGameFileConfig() (*GameFileConfig, error) {
	// 优先使用嵌入的数据
	if embeddedGameFileData != nil {
		return LoadGameFileConfigFromData(embeddedGameFileData)
	}

	// 回退：尝试从文件系统加载
	possiblePaths := []string{}

	// 1. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(cwd, "gamefile.json"))
	}

	// 2. 可执行文件目录
	if exePath, err := os.Executable(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(filepath.Dir(exePath), "gamefile.json"))
	}

	// 尝试每个路径
	for _, configPath := range possiblePaths {
		if data, err := os.ReadFile(configPath); err == nil {
			var config GameFileConfig
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("解析 gamefile.json 失败: %v", err)
			}
			logger.Debugf("加载 gamefile.json: %s", configPath)
			return &config, nil
		}
	}

	return nil, fmt.Errorf("未找到 gamefile.json")
}

// CheckBepInExFiles 检查游戏目录下是否存在 BepInEx 相关文件
func CheckBepInExFiles(gamePath string) (bool, []string) {
	config, err := LoadGameFileConfig()
	if err != nil {
		logger.Warnf("加载 gamefile.json 失败: %v", err)
		return false, nil
	}

	var missing []string
	for _, file := range config.BepInEx {
		path := filepath.Join(gamePath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, file)
		}
	}

	installed := len(missing) == 0
	return installed, missing
}

// RemoveBepInEx 移除 BepInEx，保留游戏原始文件
func RemoveBepInEx(gamePath string) error {
	logger.Infof("开始移除 BepInEx: %s", gamePath)

	config, err := LoadGameFileConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 构建需要保留的文件集合（游戏原始文件）
	keepFiles := make(map[string]bool)
	for _, file := range config.GameFile {
		keepFiles[file] = true
	}

	// 删除 BepInEx 相关文件
	for _, file := range config.BepInEx {
		path := filepath.Join(gamePath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // 文件不存在，跳过
		}

		if strings.HasSuffix(file, "/") {
			// 目录
			if err := os.RemoveAll(path); err != nil {
				logger.Errorf("删除目录失败: %s, %v", path, err)
				return fmt.Errorf("删除 %s 失败: %v", file, err)
			}
			logger.Infof("已删除目录: %s", file)
		} else {
			// 文件
			if err := os.Remove(path); err != nil {
				logger.Errorf("删除文件失败: %s, %v", path, err)
				return fmt.Errorf("删除 %s 失败: %v", file, err)
			}
			logger.Infof("已删除文件: %s", file)
		}
	}

	// 额外清理：删除 gamefile.json 中未列出的 BepInEx 相关文件
	// 但保留游戏原始文件
	entries, err := os.ReadDir(gamePath)
	if err != nil {
		logger.Warnf("读取游戏目录失败: %v", err)
	} else {
		for _, entry := range entries {
			name := entry.Name()
			// 跳过游戏原始文件
			if keepFiles[name] || keepFiles[name+"/"] {
				continue
			}
			// 跳过配置文件本身
			if name == "gamefile.json" {
				continue
			}
			// 删除其他文件（可能是 BepInEx 残留）
			path := filepath.Join(gamePath, name)
			if entry.IsDir() {
				os.RemoveAll(path)
				logger.Infof("清理残留目录: %s", name)
			} else {
				os.Remove(path)
				logger.Infof("清理残留文件: %s", name)
			}
		}
	}

	logger.Info("BepInEx 移除完成")
	return nil
}
