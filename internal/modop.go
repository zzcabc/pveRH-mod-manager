package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// ModOperator Mod 操作器，集成状态管理和备份
type ModOperator struct {
	gamePath   string
	modLibPath string
	state      *StateManager
	backup     *BackupManager
}

// NewModOperator 创建 Mod 操作器
func NewModOperator(gamePath, modLibPath string) *ModOperator {
	return &ModOperator{
		gamePath:   gamePath,
		modLibPath: modLibPath,
		state:      NewStateManager(gamePath),
		backup:     NewBackupManager(gamePath),
	}
}

// GetState 返回状态管理器
func (op *ModOperator) GetState() *StateManager {
	return op.state
}

// EnableMod 启用单个 Mod
func (op *ModOperator) EnableMod(mod AvailableMod) error {
	logger.Infof("启用 Mod: %s", mod.Name)

	// 备份现有文件
	backups, err := op.backup.BackupFiles(mod.DllNames)
	if err != nil {
		return fmt.Errorf("备份失败: %v", err)
	}

	// 依赖检查
	if err := op.ensureDependency("CustomizeLib.BepInEx.dll"); err != nil {
		op.backup.Rollback(backups)
		return err
	}

	// 复制 dll 文件
	pluginsDir := filepath.Join(op.gamePath, "BepInEx", "plugins")
	for _, dllName := range mod.DllNames {
		srcPath := op.findDll(mod.DirPath, dllName)
		if srcPath == "" {
			op.backup.Rollback(backups)
			return fmt.Errorf("未找到 dll: %s", dllName)
		}

		destPath := filepath.Join(pluginsDir, dllName)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			op.backup.Rollback(backups)
			return fmt.Errorf("读取 %s 失败: %v", dllName, err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			op.backup.Rollback(backups)
			return fmt.Errorf("写入 %s 失败: %v", dllName, err)
		}
	}

	// 更新状态
	for _, dllName := range mod.DllNames {
		op.state.Enable(dllName)
	}
	if err := op.state.Save(); err != nil {
		logger.Warnf("保存状态失败: %v", err)
	}

	// 清理备份
	op.backup.CleanBackups(mod.DllNames)
	logger.Infof("Mod 启用成功: %s", mod.Name)
	return nil
}

// DisableMod 禁用单个 Mod（不删除文件，仅更新状态）
func (op *ModOperator) DisableMod(mod AvailableMod) error {
	logger.Infof("禁用 Mod: %s", mod.Name)

	// 更新状态
	for _, dllName := range mod.DllNames {
		op.state.Disable(dllName)
	}
	if err := op.state.Save(); err != nil {
		return fmt.Errorf("保存状态失败: %v", err)
	}

	logger.Infof("Mod 禁用成功: %s", mod.Name)
	return nil
}

// RemoveMod 物理删除 Mod 文件
func (op *ModOperator) RemoveMod(mod AvailableMod) error {
	logger.Infof("删除 Mod: %s", mod.Name)

	pluginsDir := filepath.Join(op.gamePath, "BepInEx", "plugins")
	for _, dllName := range mod.DllNames {
		path := filepath.Join(pluginsDir, dllName)
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("删除 %s 失败: %v", dllName, err)
			}
		}
	}

	// 更新状态
	for _, dllName := range mod.DllNames {
		op.state.Disable(dllName)
	}
	if err := op.state.Save(); err != nil {
		logger.Warnf("保存状态失败: %v", err)
	}

	logger.Infof("Mod 删除成功: %s", mod.Name)
	return nil
}

// DeployAll 根据 enabled.txt 重新部署所有 Mod
func (op *ModOperator) DeployAll() error {
	logger.Info("开始重新部署所有 Mod")

	// 获取所有可用 Mod
	mods, err := ScanModLibrary(op.modLibPath)
	if err != nil {
		return fmt.Errorf("扫描 Mod 库失败: %v", err)
	}

	// 获取启用状态
	enabledSet := op.state.GetEnabledSet()

	// 清空 plugins 目录（保留依赖库和状态文件）
	pluginsDir := filepath.Join(op.gamePath, "BepInEx", "plugins")
	if err := op.cleanPluginsDir(pluginsDir); err != nil {
		return fmt.Errorf("清理 plugins 目录失败: %v", err)
	}

	// 按顺序重新复制启用的 Mod
	var errors []string
	for _, mod := range mods {
		if mod.IsZip {
			continue
		}

		// 检查是否所有 dll 都启用
		allEnabled := true
		for _, dll := range mod.DllNames {
			if !enabledSet[dll] {
				allEnabled = false
				break
			}
		}

		if allEnabled {
			if err := op.copyModFiles(mod); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", mod.Name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分 Mod 部署失败:\n%s", strings.Join(errors, "\n"))
	}

	logger.Info("所有 Mod 部署完成")
	return nil
}

// GetModStatus 返回 Mod 状态列表（用于 API）
func (op *ModOperator) GetModStatus() (map[string]interface{}, error) {
	installedDlls, _ := GetInstalledMods(op.gamePath)
	available, _ := ScanModLibrary(op.modLibPath)

	enabledSet := op.state.GetEnabledSet()

	dllToChinese := make(map[string]string)
	for _, m := range available {
		if m.IsZip {
			continue
		}
		for _, dll := range m.DllNames {
			dllToChinese[strings.ToLower(dll)] = m.Name
		}
	}

	installedModMap := make(map[string][]string)
	for _, dll := range installedDlls {
		lower := strings.ToLower(dll)
		cnName, ok := dllToChinese[lower]
		if !ok {
			cnName = dll
		}
		installedModMap[cnName] = append(installedModMap[cnName], dll)
	}

	installedSet := make(map[string]bool)
	var installedPlant, installedZombie []map[string]interface{}

	for cnName, dlls := range installedModMap {
		installedSet[cnName] = true
		// 检查是否启用
		enabled := true
		for _, dll := range dlls {
			if !enabledSet[dll] {
				enabled = false
				break
			}
		}
		entry := map[string]interface{}{
			"chinese_name": cnName,
			"dll_names":    dlls,
			"enabled":      enabled,
		}
		if IsZombieMod(cnName, dlls) {
			installedZombie = append(installedZombie, entry)
		} else {
			installedPlant = append(installedPlant, entry)
		}
	}

	var notInstalledPlant, notInstalledZombie []map[string]interface{}
	for _, m := range available {
		if m.IsZip {
			continue
		}
		if !installedSet[m.Name] {
			entry := map[string]interface{}{
				"name":         m.Name,
				"dll_names":    m.DllNames,
				"needs_format": NeedsFormat(m.DirPath),
				"enabled":      false,
			}
			if IsZombieMod(m.Name, m.DllNames) {
				notInstalledZombie = append(notInstalledZombie, entry)
			} else {
				notInstalledPlant = append(notInstalledPlant, entry)
			}
		}
	}

	var zips []map[string]interface{}
	for _, m := range available {
		if m.IsZip {
			zips = append(zips, map[string]interface{}{
				"name": m.Name,
			})
		}
	}

	return map[string]interface{}{
		"installed_plant":      installedPlant,
		"installed_zombie":     installedZombie,
		"not_installed_plant":  notInstalledPlant,
		"not_installed_zombie": notInstalledZombie,
		"zips":                 zips,
	}, nil
}

// ensureDependency 确保依赖库存在
func (op *ModOperator) ensureDependency(dllName string) error {
	pluginsDir := filepath.Join(op.gamePath, "BepInEx", "plugins")
	targetPath := filepath.Join(pluginsDir, dllName)

	if _, err := os.Stat(targetPath); err == nil {
		return nil // 已存在
	}

	// 在 Mod 库中查找
	var foundPath string
	filepath.Walk(op.modLibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(info.Name(), dllName) {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})

	if foundPath == "" {
		return fmt.Errorf("缺少依赖 %s 且在 Mod 库中未找到", dllName)
	}

	data, err := os.ReadFile(foundPath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0644)
}

// findDll 在 Mod 目录中查找 dll 文件
func (op *ModOperator) findDll(modDir, dllName string) string {
	for _, sub := range []string{modDir, filepath.Join(modDir, "plugins"), filepath.Join(modDir, "BepInEx", "plugins")} {
		p := filepath.Join(sub, dllName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// cleanPluginsDir 清空 plugins 目录（保留依赖库和状态文件）
func (op *ModOperator) cleanPluginsDir(pluginsDir string) error {
	keepFiles := map[string]bool{
		"enabled.txt":              true,
		"CustomizeLib.BepInEx.dll": true,
		"SkinLoader.dll":           true,
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if keepFiles[e.Name()] {
			continue
		}
		os.Remove(filepath.Join(pluginsDir, e.Name()))
	}
	return nil
}

// copyModFiles 复制 Mod 文件到 plugins 目录
func (op *ModOperator) copyModFiles(mod AvailableMod) error {
	pluginsDir := filepath.Join(op.gamePath, "BepInEx", "plugins")
	for _, dllName := range mod.DllNames {
		srcPath := op.findDll(mod.DirPath, dllName)
		if srcPath == "" {
			return fmt.Errorf("未找到 dll: %s", dllName)
		}

		destPath := filepath.Join(pluginsDir, dllName)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
