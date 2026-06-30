package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ===== 数据结构 =====

// Config 运行时配置
type Config struct {
	GamePath     []GameEntry `json:"game_path"`
	ModPath      []string    `json:"mod_path"`
	DownloadPath string      `json:"download_path"`
	ServerURL    string      `json:"server_url"`
}

// GameEntry 游戏目录条目
type GameEntry struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// GameFileManifest gamefile.json 的结构
type GameFileManifest struct {
	GameFile []string `json:"GameFile"`
	BepInEx  []string `json:"BepInEx"`
}

// ===== 常量 =====

const (
	configFileName   = "config.json"
	gamefileFileName = "gamefile.json"
	defaultServerURL = "https://pvzrh.zhaocheng.cc:8443"
)

// 版本目录名正则：匹配 作者-版本、裸版本号、作者-通用
var versionDirPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^.+?-(\d+\.\d+(?:\.\d+)?)$`), // 高数羽衫-3.7
	regexp.MustCompile(`^(\d+\.\d+(?:\.\d+)?)$`),     // 3.7
}

var genericPattern = regexp.MustCompile(`^.+?-通用$`)

// ===== 配置读写 =====

// LoadConfig 读取 config.json，不存在则创建默认
func LoadConfig() (*Config, error) {
	configPath := filepath.Join(exeDir(), configFileName)

	if !FileExists(configPath) {
		return createDefaultConfig(configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 补全缺失字段
	if cfg.ServerURL == "" {
		cfg.ServerURL = defaultServerURL
	}

	return &cfg, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config) error {
	configPath := filepath.Join(exeDir(), configFileName)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// createDefaultConfig 创建默认配置
func createDefaultConfig(path string) (*Config, error) {
	cfg := &Config{
		GamePath:     []GameEntry{},
		ModPath:      []string{},
		DownloadPath: "",
		ServerURL:    defaultServerURL,
	}

	if err := SaveConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadGameFileManifest 读取项目内置的 gamefile.json
func LoadGameFileManifest() (*GameFileManifest, error) {
	// 优先从程序目录读取
	path := filepath.Join(exeDir(), gamefileFileName)
	if !FileExists(path) {
		// 开发时从当前目录读取
		path = gamefileFileName
		if !FileExists(path) {
			return nil, fmt.Errorf("未找到 %s 文件", gamefileFileName)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", gamefileFileName, err)
	}

	var manifest GameFileManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", gamefileFileName, err)
	}

	return &manifest, nil
}

// ===== 版本检测 =====

// DetectVersions 扫描所有 mod_path 目录，提取可用的游戏版本号列表
func DetectVersions(modPaths []string) []string {
	versionSet := make(map[string]bool)

	for _, modPath := range modPaths {
		if !DirExists(modPath) {
			continue
		}

		authorDirs, err := os.ReadDir(modPath)
		if err != nil {
			continue
		}

		for _, authorDir := range authorDirs {
			if !authorDir.IsDir() {
				continue
			}

			versionDirs, err := os.ReadDir(filepath.Join(modPath, authorDir.Name()))
			if err != nil {
				continue
			}

			for _, verDir := range versionDirs {
				if !verDir.IsDir() {
					continue
				}

				ver := ExtractVersion(verDir.Name())
				if ver != "" {
					versionSet[ver] = true
				}
			}
		}
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	return versions
}

// ExtractVersion 从版本目录名中提取版本号
// "高数羽衫-3.7" → "3.7"
// "3.6.1" → "3.6.1"
// "高数羽衫-通用" → "通用"
func ExtractVersion(dirName string) string {
	if genericPattern.MatchString(dirName) {
		return "通用"
	}

	for _, pattern := range versionDirPatterns {
		matches := pattern.FindStringSubmatch(dirName)
		if len(matches) >= 2 {
			return matches[1]
		}
	}

	return ""
}

// MatchVersionDir 判断版本目录名是否匹配目标版本
// 匹配逻辑：版本目录的版本号 == 目标版本，或者版本目录为"通用"
func MatchVersionDir(dirName, targetVersion string) bool {
	ver := ExtractVersion(dirName)
	if ver == "" {
		return false
	}
	if ver == "通用" {
		return true
	}
	return ver == targetVersion
}

// ===== 辅助函数 =====

// exeDir 获取可执行文件所在目录
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// ===== 显示名解析 =====

// ParseDisplayName 从 MOD 目录名提取中文显示名
// "GiantFlameBambooLoong-巨焰竹魁龙" → "巨焰竹魁龙"
// "(金银钻)杨桃" → "(金银钻)杨桃"
// "Compound Z-复合物 Z" → "复合物 Z"
func ParseDisplayName(dirName string) string {
	// 找到最后一个 "-" 后面的内容
	lastDash := strings.LastIndex(dirName, "-")
	if lastDash >= 0 && lastDash < len(dirName)-1 {
		suffix := dirName[lastDash+1:]
		// 如果后缀包含中文，视为中文名
		if containsChinese(suffix) {
			return suffix
		}
		// 如果后缀看起来不像中文名（太短或无中文），返回原名
		if len(suffix) <= 2 {
			return dirName
		}
		return suffix
	}
	return dirName
}

// containsChinese 判断字符串是否包含中文字符
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}
