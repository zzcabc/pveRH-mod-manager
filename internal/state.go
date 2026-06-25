package internal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"pveRH-mod-manager/internal/logger"
)

// StateManager 管理 Mod 启用状态
type StateManager struct {
	mu       sync.RWMutex
	gamePath string
	enabled  map[string]bool // mod名 -> 是否启用
}

// NewStateManager 创建状态管理器
func NewStateManager(gamePath string) *StateManager {
	sm := &StateManager{
		gamePath: gamePath,
		enabled:  make(map[string]bool),
	}
	sm.load()
	return sm
}

// stateFilePath 返回 enabled.txt 路径
func (sm *StateManager) stateFilePath() string {
	return filepath.Join(sm.gamePath, "BepInEx", "plugins", "enabled.txt")
}

// load 从文件加载启用状态
func (sm *StateManager) load() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	path := sm.stateFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在时，将当前所有 dll 视为已启用（兼容旧版本）
		logger.Debug("enabled.txt 不存在，初始化为空")
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			sm.enabled[line] = true
		}
	}
	logger.Infof("加载启用状态: %d 个 Mod", len(sm.enabled))
}

// Save 保存启用状态到文件
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	path := sm.stateFilePath()
	os.MkdirAll(filepath.Dir(path), os.ModePerm)

	// 排序后写入，便于版本控制
	names := make([]string, 0, len(sm.enabled))
	for name := range sm.enabled {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("# 已启用的 Mod 列表\n")
	sb.WriteString("# 每行一个 dll 文件名\n\n")
	for _, name := range names {
		sb.WriteString(name)
		sb.WriteString("\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// IsEnabled 检查 Mod 是否启用
func (sm *StateManager) IsEnabled(modName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.enabled[modName]
}

// Enable 启用 Mod
func (sm *StateManager) Enable(modName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.enabled[modName] = true
}

// Disable 禁用 Mod
func (sm *StateManager) Disable(modName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.enabled, modName)
}

// GetEnabled 返回所有已启用的 Mod 名称
func (sm *StateManager) GetEnabled() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.enabled))
	for name := range sm.enabled {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetEnabledSet 返回已启用 Mod 的集合（用于批量查询）
func (sm *StateManager) GetEnabledSet() map[string]bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]bool, len(sm.enabled))
	for k, v := range sm.enabled {
		result[k] = v
	}
	return result
}

// BatchEnable 批量启用 Mod
func (sm *StateManager) BatchEnable(modNames []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, name := range modNames {
		sm.enabled[name] = true
	}
}

// BatchDisable 批量禁用 Mod
func (sm *StateManager) BatchDisable(modNames []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, name := range modNames {
		delete(sm.enabled, name)
	}
}

// SyncFromDisk 从磁盘同步当前已安装的 dll 到启用状态
// 用于首次运行或迁移场景
func (sm *StateManager) SyncFromDisk() error {
	pluginsDir := filepath.Join(sm.gamePath, "BepInEx", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("读取 plugins 目录失败: %v", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			// 跳过依赖库
			if strings.EqualFold(e.Name(), "CustomizeLib.BepInEx.dll") ||
				strings.EqualFold(e.Name(), "SkinLoader.dll") {
				continue
			}
			sm.enabled[e.Name()] = true
		}
	}

	logger.Infof("从磁盘同步 %d 个 Mod 到启用状态", len(sm.enabled))
	return sm.Save()
}

// HasStateFile 检查是否存在状态文件
func (sm *StateManager) HasStateFile() bool {
	_, err := os.Stat(sm.stateFilePath())
	return err == nil
}
