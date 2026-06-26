package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// GamePath 游戏目录配置（带版本号）
type GamePath struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// Config 程序配置
type Config struct {
	GamePaths    []GamePath `json:"game_path"`
	ModPaths     []string   `json:"mod_path"`
	DownloadPath string     `json:"download_path"`
	ServerURL    string     `json:"server_url"`
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     Config
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// Load 加载配置
func (cm *ConfigManager) Load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("配置文件不存在，使用默认配置")
			return nil
		}
		return err
	}

	// 尝试解析新格式
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		logger.Warn("解析配置失败，尝试兼容旧格式")
		return cm.loadLegacyFormat(data)
	}

	cm.config = config
	logger.Infof("加载配置成功: %d 个游戏目录, %d 个 MOD 目录",
		len(cm.config.GamePaths), len(cm.config.ModPaths))
	return nil
}

// loadLegacyFormat 兼容旧格式配置
func (cm *ConfigManager) loadLegacyFormat(data []byte) error {
	var legacy struct {
		GamePaths    []string `json:"game_paths"`
		ModLibPaths  []string `json:"modlib_paths"`
		DownloadPath string   `json:"download_path"`
	}

	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}

	// 转换为新格式
	cm.config.GamePaths = make([]GamePath, 0, len(legacy.GamePaths))
	for _, path := range legacy.GamePaths {
		version := DetectVersionFromPath(path)
		cm.config.GamePaths = append(cm.config.GamePaths, GamePath{
			Path:    path,
			Version: version,
		})
	}
	cm.config.ModPaths = legacy.ModLibPaths
	cm.config.DownloadPath = legacy.DownloadPath

	logger.Info("已将旧格式配置转换为新格式")
	return nil
}

// Save 保存配置
func (cm *ConfigManager) Save() error {
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.configPath, data, 0644)
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() Config {
	return cm.config
}

// GetGamePaths 获取所有游戏目录
func (cm *ConfigManager) GetGamePaths() []GamePath {
	return cm.config.GamePaths
}

// GetModPaths 获取所有 MOD 目录
func (cm *ConfigManager) GetModPaths() []string {
	return cm.config.ModPaths
}

// GetDownloadPath 获取下载目录
func (cm *ConfigManager) GetDownloadPath() string {
	return cm.config.DownloadPath
}

// SetDownloadPath 设置下载目录
func (cm *ConfigManager) SetDownloadPath(path string) {
	cm.config.DownloadPath = path
}

const defaultServerURL = "https://pvzrhmod.zhaocheng.cc:8443"

// GetServerURL 获取服务器 URL（未配置时返回默认值）
func (cm *ConfigManager) GetServerURL() string {
	if cm.config.ServerURL != "" {
		return cm.config.ServerURL
	}
	return defaultServerURL
}

// SetServerURL 设置服务器 URL
func (cm *ConfigManager) SetServerURL(url string) {
	cm.config.ServerURL = url
}

// AddGamePath 添加游戏目录
func (cm *ConfigManager) AddGamePath(path string) {
	// 检查是否已存在
	for _, gp := range cm.config.GamePaths {
		if gp.Path == path {
			return
		}
	}

	version := DetectVersionFromPath(path)
	cm.config.GamePaths = append(cm.config.GamePaths, GamePath{
		Path:    path,
		Version: version,
	})
	logger.Infof("添加游戏目录: %s (版本: %s)", path, version)
}

// AddModPath 添加 MOD 目录
func (cm *ConfigManager) AddModPath(path string) {
	for _, p := range cm.config.ModPaths {
		if p == path {
			return
		}
	}
	cm.config.ModPaths = append(cm.config.ModPaths, path)
	logger.Infof("添加 MOD 目录: %s", path)
}

// RemoveGamePath 移除游戏目录
func (cm *ConfigManager) RemoveGamePath(path string) {
	for i, gp := range cm.config.GamePaths {
		if gp.Path == path {
			cm.config.GamePaths = append(cm.config.GamePaths[:i], cm.config.GamePaths[i+1:]...)
			logger.Infof("移除游戏目录: %s", path)
			return
		}
	}
}

// RemoveModPath 移除 MOD 目录
func (cm *ConfigManager) RemoveModPath(path string) {
	for i, p := range cm.config.ModPaths {
		if p == path {
			cm.config.ModPaths = append(cm.config.ModPaths[:i], cm.config.ModPaths[i+1:]...)
			logger.Infof("移除 MOD 目录: %s", path)
			return
		}
	}
}

// GetVersions 获取所有可用版本
func (cm *ConfigManager) GetVersions() []string {
	versionSet := make(map[string]bool)
	for _, gp := range cm.config.GamePaths {
		if gp.Version != "" {
			versionSet[gp.Version] = true
		}
	}

	// 从 MOD 目录检测版本
	for _, modPath := range cm.config.ModPaths {
		versions := DetectVersionsFromModPath(modPath)
		for _, v := range versions {
			versionSet[v] = true
		}
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}

	// 版本排序（降序，新版本在前）
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions
}

// GetGamePathsByVersion 根据版本获取游戏目录
func (cm *ConfigManager) GetGamePathsByVersion(version string) []string {
	var paths []string
	for _, gp := range cm.config.GamePaths {
		if gp.Version == version {
			paths = append(paths, gp.Path)
		}
	}
	return paths
}

// DetectVersionFromPath 从路径中检测版本号
func DetectVersionFromPath(path string) string {
	// 匹配常见的版本号格式：3.7, 3.6.1, v3.7 等
	re := regexp.MustCompile(`(?:v)?(\d+\.\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(filepath.Base(path))
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// DetectVersionsFromModPath 从 MOD 目录检测所有可用版本
func DetectVersionsFromModPath(modPath string) []string {
	versionSet := make(map[string]bool)

	entries, err := os.ReadDir(modPath)
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`(?:v)?(\d+\.\d+(?:\.\d+)?)`)
	for _, entry := range entries {
		if entry.IsDir() {
			matches := re.FindStringSubmatch(entry.Name())
			if len(matches) > 1 {
				versionSet[matches[1]] = true
			}
		}
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}
	return versions
}

// compareVersions 比较版本号，返回 1, 0, -1
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parseVersionPart(parts1[i])
		}
		if i < len(parts2) {
			p2 = parseVersionPart(parts2[i])
		}

		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}
	return 0
}

func parseVersionPart(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
