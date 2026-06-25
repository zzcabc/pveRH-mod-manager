package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pveRH-mod-manager/internal/logger"
)

// BackupManager 管理 Mod 备份和恢复
type BackupManager struct {
	gamePath string
}

// NewBackupManager 创建备份管理器
func NewBackupManager(gamePath string) *BackupManager {
	return &BackupManager{gamePath: gamePath}
}

// backupDir 返回备份目录路径
func (bm *BackupManager) backupDir() string {
	return filepath.Join(bm.gamePath, "BepInEx", "plugins", "backup")
}

// BackupFile 备份单个文件，返回备份路径
func (bm *BackupManager) BackupFile(fileName string) (string, error) {
	srcPath := filepath.Join(bm.gamePath, "BepInEx", "plugins", fileName)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", nil // 文件不存在，无需备份
	}

	backupDir := bm.backupDir()
	os.MkdirAll(backupDir, os.ModePerm)

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s.%s.bak", fileName, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("写入备份失败: %v", err)
	}

	logger.Debugf("备份文件: %s -> %s", fileName, backupName)
	return backupPath, nil
}

// BackupFiles 批量备份文件，返回备份映射
func (bm *BackupManager) BackupFiles(fileNames []string) (map[string]string, error) {
	backups := make(map[string]string)
	for _, name := range fileNames {
		backupPath, err := bm.BackupFile(name)
		if err != nil {
			// 回滚已备份的文件
			bm.Rollback(backups)
			return nil, fmt.Errorf("备份 %s 失败: %v", name, err)
		}
		if backupPath != "" {
			backups[name] = backupPath
		}
	}
	return backups, nil
}

// Rollback 从备份恢复文件
func (bm *BackupManager) Rollback(backups map[string]string) {
	pluginsDir := filepath.Join(bm.gamePath, "BepInEx", "plugins")
	for fileName, backupPath := range backups {
		if backupPath == "" {
			continue
		}
		data, err := os.ReadFile(backupPath)
		if err != nil {
			logger.Errorf("读取备份失败: %s, %v", backupPath, err)
			continue
		}
		destPath := filepath.Join(pluginsDir, fileName)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			logger.Errorf("恢复备份失败: %s, %v", fileName, err)
		} else {
			logger.Debugf("恢复备份: %s", fileName)
		}
	}
}

// CleanBackups 清理指定文件的备份
func (bm *BackupManager) CleanBackups(fileNames []string) {
	backupDir := bm.backupDir()
	for _, name := range fileNames {
		pattern := filepath.Join(backupDir, name+".*.bak")
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			os.Remove(match)
		}
	}
}

// CleanAllBackups 清理所有备份
func (bm *BackupManager) CleanAllBackups() {
	backupDir := bm.backupDir()
	os.RemoveAll(backupDir)
}

// OperationResult 单个操作的结果
type OperationResult struct {
	ModName string
	Success bool
	Error   error
}

// BatchResult 批量操作结果
type BatchResult struct {
	Results []OperationResult
	Errors  []error
}

// HasErrors 检查是否有错误
func (br *BatchResult) HasErrors() bool {
	return len(br.Errors) > 0
}

// ErrorSummary 返回错误摘要
func (br *BatchResult) ErrorSummary() string {
	if len(br.Errors) == 0 {
		return ""
	}
	msg := fmt.Sprintf("%d 个操作失败:\n", len(br.Errors))
	for _, err := range br.Errors {
		msg += "  - " + err.Error() + "\n"
	}
	return msg
}
