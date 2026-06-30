package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ===== 修改器模型 =====

// ModifierPack 修改器包信息
type ModifierPack struct {
	FileName   string `json:"file_name"`   // zip 文件名
	Version    string `json:"version"`     // 匹配的版本
	Author     string `json:"author"`      // 所属作者
	SourcePath string `json:"source_path"` // zip 完整路径
}

// ===== 修改器扫描 =====

// FindModifier 在 MOD 目录中搜索匹配目标版本的修改器 zip
// 搜索路径：{modPath}/{作者}/{作者}-{version}/修改器/
func FindModifier(modPath string, version string) (*ModifierPack, error) {
	if !DirExists(modPath) {
		return nil, fmt.Errorf("MOD 目录不存在: %s", modPath)
	}

	authorDirs, err := os.ReadDir(modPath)
	if err != nil {
		return nil, fmt.Errorf("读取 MOD 目录失败: %w", err)
	}

	for _, authorDir := range authorDirs {
		if !authorDir.IsDir() {
			continue
		}
		author := authorDir.Name()
		authorPath := filepath.Join(modPath, author)

		verDirs, err := os.ReadDir(authorPath)
		if err != nil {
			continue
		}

		for _, verDir := range verDirs {
			if !verDir.IsDir() {
				continue
			}

			if !MatchVersionDir(verDir.Name(), version) {
				continue
			}

			verPath := filepath.Join(authorPath, verDir.Name())
			modifierDir := filepath.Join(verPath, "修改器")
			if !DirExists(modifierDir) {
				continue
			}

			// 遍历修改器子目录
			modifierSubDirs, err := os.ReadDir(modifierDir)
			if err != nil {
				continue
			}

			for _, subDir := range modifierSubDirs {
				if !subDir.IsDir() {
					// 也可能直接在修改器目录下有 zip
					if strings.HasSuffix(strings.ToLower(subDir.Name()), ".zip") {
						zipPath := filepath.Join(modifierDir, subDir.Name())
						if matchModifierZip(subDir.Name(), version) {
							return &ModifierPack{
								FileName:   subDir.Name(),
								Version:    version,
								Author:     author,
								SourcePath: zipPath,
							}, nil
						}
					}
					continue
				}

				// 在子目录中查找 zip
				subPath := filepath.Join(modifierDir, subDir.Name())
				files, err := os.ReadDir(subPath)
				if err != nil {
					continue
				}

				for _, f := range files {
					if f.IsDir() {
						continue
					}
					if !strings.HasSuffix(strings.ToLower(f.Name()), ".zip") {
						continue
					}
					if matchModifierZip(f.Name(), version) {
						return &ModifierPack{
							FileName:   f.Name(),
							Version:    version,
							Author:     author,
							SourcePath: filepath.Join(subPath, f.Name()),
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("未找到匹配版本 %s 的修改器 zip", version)
}

// FindAllModifiers 搜索所有可用的修改器（不限版本）
func FindAllModifiers(modPath string) []ModifierPack {
	var packs []ModifierPack

	if !DirExists(modPath) {
		return packs
	}

	authorDirs, _ := os.ReadDir(modPath)
	for _, authorDir := range authorDirs {
		if !authorDir.IsDir() {
			continue
		}
		author := authorDir.Name()
		authorPath := filepath.Join(modPath, author)

		verDirs, _ := os.ReadDir(authorPath)
		for _, verDir := range verDirs {
			if !verDir.IsDir() {
				continue
			}
			ver := ExtractVersion(verDir.Name())
			verPath := filepath.Join(authorPath, verDir.Name())
			modifierDir := filepath.Join(verPath, "修改器")
			if !DirExists(modifierDir) {
				continue
			}

			// 递归查找 zip
			filepath.Walk(modifierDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(strings.ToLower(info.Name()), ".zip") &&
					matchModifierZip(info.Name(), ver) {
					packs = append(packs, ModifierPack{
						FileName:   info.Name(),
						Version:    ver,
						Author:     author,
						SourcePath: path,
					})
				}
				return nil
			})
		}
	}

	return packs
}

// ===== 修改器安装 =====

// InstallModifier 安装修改器：解压到游戏目录
func InstallModifier(pack ModifierPack, gamePath string) error {
	fmt.Printf("安装修改器: %s (版本 %s, 作者 %s)\n", pack.FileName, pack.Version, pack.Author)
	fmt.Printf("解压到: %s\n", gamePath)

	if err := Unzip(pack.SourcePath, gamePath); err != nil {
		return fmt.Errorf("安装修改器失败: %w", err)
	}

	return nil
}

// ===== 辅助 =====

// matchModifierZip 判断文件名是否匹配 PvZRHModfiedFor{版本}[].zip 格式
func matchModifierZip(fileName, version string) bool {
	name := strings.ToLower(fileName)

	// 标准格式：PvZRHModfiedFor3.7【...】.zip
	expected := strings.ToLower(fmt.Sprintf("PvZRHModfiedFor%s", version))
	if strings.Contains(name, expected) {
		return true
	}

	// 不带版本后缀的格式（如 PvZRHModfied【26-6-28更新】.exe）
	// 只要包含 PvZRHModfied 即视为修改器
	if strings.Contains(name, strings.ToLower("PvZRHModfied")) {
		return true
	}

	return false
}
