package internal

import (
	"encoding/json"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// ModEntry 单个 MOD 条目
type ModEntry struct {
	ChineseName string `json:"chineseName"`
	ModName     string `json:"modName"`
}

// AuthorModInfo 作者的 MOD 信息
type AuthorModInfo struct {
	Author     string     `json:"author"`
	Version    string     `json:"version"`
	PlanList   []ModEntry `json:"planList"`
	ZombieList []ModEntry `json:"zombieList"`
	SkinList   []ModEntry `json:"skinList"`
	PluginList []ModEntry `json:"pluginList"`
}

// ModInfoManager MOD 信息管理器
type ModInfoManager struct {
	infos []AuthorModInfo
	// 按版本和 dll 名称索引：version -> dllName(lower) -> ModEntry
	index map[string]map[string]ModEntry
	// 按版本和中文名索引：version -> chineseName -> ModEntry
	chineseIndex map[string]map[string]ModEntry
}

// 全局变量，由 main 包初始化
var embeddedModInfoData []byte
var modInfoManager *ModInfoManager

// SetEmbeddedModInfoData 设置嵌入的 mod.json 数据
func SetEmbeddedModInfoData(data []byte) {
	embeddedModInfoData = data
	var err error
	modInfoManager, err = NewModInfoManager(data)
	if err != nil {
		logger.Errorf("初始化 ModInfoManager 失败: %v", err)
	}
}

// NewModInfoManager 创建 MOD 信息管理器
func NewModInfoManager(data []byte) (*ModInfoManager, error) {
	var infos []AuthorModInfo
	if err := json.Unmarshal(data, &infos); err != nil {
		return nil, err
	}

	mim := &ModInfoManager{
		infos:        infos,
		index:        make(map[string]map[string]ModEntry),
		chineseIndex: make(map[string]map[string]ModEntry),
	}
	mim.buildIndex()
	logger.Infof("加载 MOD 信息: %d 个作者配置", len(infos))
	return mim, nil
}

// buildIndex 构建索引
func (mim *ModInfoManager) buildIndex() {
	for _, info := range mim.infos {
		version := info.Version
		if mim.index[version] == nil {
			mim.index[version] = make(map[string]ModEntry)
			mim.chineseIndex[version] = make(map[string]ModEntry)
		}

		// 处理植物 MOD
		for _, entry := range info.PlanList {
			mim.addEntry(version, entry, "plant")
		}

		// 处理僵尸 MOD
		for _, entry := range info.ZombieList {
			mim.addEntry(version, entry, "zombie")
		}

		// 处理皮肤
		for _, entry := range info.SkinList {
			mim.addEntry(version, entry, "skin")
		}

		// 处理插件
		for _, entry := range info.PluginList {
			mim.addEntry(version, entry, "plugin")
		}
	}
}

// addEntry 添加条目到索引
func (mim *ModInfoManager) addEntry(version string, entry ModEntry, category string) {
	// 处理可能包含多个 dll 的情况（逗号分隔）
	modNames := strings.Split(entry.ModName, ",")
	for _, modName := range modNames {
		modName = strings.TrimSpace(modName)
		if modName == "" {
			continue
		}

		// 存储带分类信息的条目
		indexedEntry := ModEntry{
			ChineseName: entry.ChineseName,
			ModName:     modName,
		}

		mim.index[version][strings.ToLower(modName)] = indexedEntry
		mim.chineseIndex[version][entry.ChineseName] = indexedEntry
	}
}

// GetChineseName 根据版本和 dll 名称获取中文名
func (mim *ModInfoManager) GetChineseName(version, dllName string) string {
	if mim == nil {
		return dllName
	}

	if versionIndex, ok := mim.index[version]; ok {
		if entry, ok := versionIndex[strings.ToLower(dllName)]; ok {
			return entry.ChineseName
		}
	}

	// 尝试在所有版本中查找
	for _, versionIndex := range mim.index {
		if entry, ok := versionIndex[strings.ToLower(dllName)]; ok {
			return entry.ChineseName
		}
	}

	return dllName
}

// GetModEntry 根据版本和 dll 名称获取完整条目
func (mim *ModInfoManager) GetModEntry(version, dllName string) (ModEntry, bool) {
	if mim == nil {
		return ModEntry{}, false
	}

	if versionIndex, ok := mim.index[version]; ok {
		if entry, ok := versionIndex[strings.ToLower(dllName)]; ok {
			return entry, true
		}
	}
	return ModEntry{}, false
}

// GetModsByCategory 根据版本和分类获取 MOD 列表
func (mim *ModInfoManager) GetModsByCategory(version, category string) []ModEntry {
	if mim == nil {
		return nil
	}

	var result []ModEntry
	for _, info := range mim.infos {
		if info.Version != version {
			continue
		}

		var list []ModEntry
		switch category {
		case "plant":
			list = info.PlanList
		case "zombie":
			list = info.ZombieList
		case "skin":
			list = info.SkinList
		case "plugin":
			list = info.PluginList
		}

		result = append(result, list...)
	}
	return result
}

// GetAllVersions 获取所有版本
func (mim *ModInfoManager) GetAllVersions() []string {
	if mim == nil {
		return nil
	}

	versionSet := make(map[string]bool)
	for _, info := range mim.infos {
		versionSet[info.Version] = true
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}
	return versions
}

// GetModInfoManager 获取全局 MOD 信息管理器
func GetModInfoManager() *ModInfoManager {
	return modInfoManager
}
