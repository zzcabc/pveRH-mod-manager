package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"pveRH-mod-manager/internal/logger"
	"regexp"
	"strings"
)

// ModJSON 格式
type ModJSON struct {
	Author     string        `json:"author"`
	Version    string        `json:"version"`
	PlanList   []ModJSONItem `json:"planList"`
	ZombieList []ModJSONItem `json:"zombieList"`
	SkinList   []ModJSONItem `json:"skinList"`
	PluginList []ModJSONItem `json:"pluginList"`
}

type ModJSONItem struct {
	ChineseName string `json:"chineseName"`
	ModName     string `json:"modName"`
}

// 目标分类文件夹名
const (
	CatPlant   = "植物MOD"
	CatZombie  = "僵尸MOD"
	CatSkin    = "皮肤"
	CatPlugin  = "插件"
	CatTrainer = "修改器"
	CatPet     = "宠物MOD"
	CatOther   = "其他"
)

var (
	compatRe     = regexp.MustCompile(`兼容\s*(\d+\.\d+(?:\.\d+)?)`)
	versionRe    = regexp.MustCompile(`^3\.\d+`)
	zipVersionRe = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
)

// ============ 主入口 ============

func NormalizeDownloadFolder(downloadDir string) error {
	logger.Infof("=== 开始规范化下载文件夹 ===")
	HandleTheAuthorFolder(downloadDir)
	logger.Infof("=== 生成 mod.json ===")
	generateMergedModJSON(downloadDir)
	logger.Infof("=== 规范化完成 ===")
	return nil
}

func HandleTheAuthorFolder(downloadDir string) {
	authorDirList, err := os.ReadDir(downloadDir)
	if err != nil {
		logger.Errorf("读取作者文件夹失败: %v", err)
		return
	}

	for _, authorDir := range authorDirList {
		name := authorDir.Name()
		logger.Infof("处理作者文件夹: %s", name)
		if authorDir.IsDir() {
			switch name {
			case "高数羽衫":
				HandleTheGaoShuFolder(downloadDir, authorDir.Name())
			case "慕容孤晴":
				HandleTheMurongFolder(downloadDir, authorDir.Name())
			case "林秋鲑鱼":
				HandleTheLinQiuFolder(downloadDir, authorDir.Name())
			case "梧萱梦汐":
				HandleTheWuXuanFolder(downloadDir, authorDir.Name())
			}
		}
	}
}

func HandleTheGaoShuFolder(downloadDir string, authorDirName string) {
	authorDir := filepath.Join(downloadDir, authorDirName)
	logger.Infof("处理作者文件夹: %s", authorDir)
	entries, err := os.ReadDir(authorDir)
	if err != nil {
		logger.Errorf("读取 %s 文件夹失败: %v", authorDir, err)
		return
	}

	// 收集所有版本号
	versions := GetGaoShuModVersion(authorDir)
	logger.Infof("  发现版本: %v", versions)

	// 创建版本文件夹
	for _, ver := range versions {
		os.MkdirAll(filepath.Join(authorDir, authorDirName+"-"+ver), os.ModePerm)
	}

	// 散落文件 → 其他/
	otherDir := filepath.Join(authorDir, CatOther)
	os.MkdirAll(otherDir, os.ModePerm)
	for _, entry := range entries {
		if !entry.IsDir() {
			src := filepath.Join(authorDir, entry.Name())
			dst := filepath.Join(otherDir, entry.Name())
			os.Rename(src, dst)
			logger.Infof("  %s → 其他/", entry.Name())
		}
	}

	// 分类处理每个子文件夹
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		catName := entry.Name()
		cat := classifyGaoShuCategory(catName)
		if cat == "" {
			logger.Infof("  [跳过] %s", catName)
			continue
		}

		subEntries, _ := os.ReadDir(filepath.Join(authorDir, catName))

		// 检查是否有版本子目录
		hasVersion := false
		for _, sub := range subEntries {
			if sub.IsDir() && versionRe.MatchString(sub.Name()) {
				hasVersion = true
				break
			}
		}

		if hasVersion {
			for _, sub := range subEntries {
				if !sub.IsDir() || !versionRe.MatchString(sub.Name()) {
					continue
				}
				ver := normalizeVersion(sub.Name())
				srcDir := filepath.Join(authorDir, catName, sub.Name())

				if cat == CatOther {
					// 其他 → 作者级别
					otherDir := filepath.Join(authorDir, CatOther)
					os.MkdirAll(otherDir, os.ModePerm)
					logger.Infof("  %s/%s → 其他", catName, sub.Name())
					moveAll(srcDir, otherDir)
				} else if cat == CatTrainer {
					// 修改器 → 版本文件夹，只移动匹配的 zip
					verDir := filepath.Join(authorDir, authorDirName+"-"+ver, cat)
					os.MkdirAll(verDir, os.ModePerm)
					logger.Infof("  %s/%s → %s/%s", catName, sub.Name(), ver, cat)
					moveTrainerZips(srcDir, verDir)
				} else if cat == CatSkin {
					// 皮肤 → 作者级别，解压提取皮肤文件夹
					catDir := filepath.Join(authorDir, cat)
					os.MkdirAll(catDir, os.ModePerm)
					logger.Infof("  %s/%s → %s (作者级别)", catName, sub.Name(), cat)
					extractSkinZipsToFolder(srcDir, catDir)
				} else if cat == CatPet {
					// 宠物 → 作者级别，解压提取 dll
					catDir := filepath.Join(authorDir, cat)
					os.MkdirAll(catDir, os.ModePerm)
					logger.Infof("  %s/%s → %s (作者级别)", catName, sub.Name(), cat)
					extractZipsToFolder(srcDir, catDir)
				} else {
					// 植物、僵尸 → 版本文件夹，解压提取 dll
					verDir := filepath.Join(authorDir, authorDirName+"-"+ver, cat)
					os.MkdirAll(verDir, os.ModePerm)
					logger.Infof("  %s/%s → %s/%s", catName, sub.Name(), ver, cat)
					extractZipsToFolder(srcDir, verDir)
				}
			}
		} else {
			if cat == CatOther {
				otherDir := filepath.Join(authorDir, CatOther)
				os.MkdirAll(otherDir, os.ModePerm)
				logger.Infof("  %s → 其他", catName)
				moveAll(filepath.Join(authorDir, catName), otherDir)
			} else if cat == CatSkin {
				catDir := filepath.Join(authorDir, cat)
				os.MkdirAll(catDir, os.ModePerm)
				logger.Infof("  %s → %s (作者级别)", catName, cat)
				extractSkinZipsToFolder(filepath.Join(authorDir, catName), catDir)
			} else if cat == CatPet {
				catDir := filepath.Join(authorDir, cat)
				os.MkdirAll(catDir, os.ModePerm)
				logger.Infof("  %s → %s (作者级别)", catName, cat)
				extractZipsToFolder(filepath.Join(authorDir, catName), catDir)
			} else {
				for _, ver := range versions {
					verDir := filepath.Join(authorDir, authorDirName+"-"+ver, cat)
					os.MkdirAll(verDir, os.ModePerm)
					logger.Infof("  %s → %s/%s (所有版本)", catName, ver, cat)
					extractZipsToFolder(filepath.Join(authorDir, catName), verDir)
				}
			}
		}
	}

	// 清理：保留版本文件夹和作者级别分类文件夹
	keep := map[string]bool{
		CatPet:   true,
		CatSkin:  true,
		CatOther: true,
	}
	for _, ver := range versions {
		keep[authorDirName+"-"+ver] = true
	}
	logger.Infof("  白名单: %v", keep)
	for _, entry := range entries {
		if entry.IsDir() {
			if keep[entry.Name()] {
				logger.Infof("  [保留] %s", entry.Name())
			} else {
				os.RemoveAll(filepath.Join(authorDir, entry.Name()))
				logger.Infof("  [清理] %s/", entry.Name())
			}
		}
	}
}

// HandleTheMurongFolder 处理慕容孤晴文件夹
// 结构：3.6.1/BepInEx二创植物（3.6.1）/xxx.zip
// 目标：慕容孤晴-版本/植物MOD/中文名/dll
func HandleTheMurongFolder(downloadDir string, authorDirName string) {
	authorDir := filepath.Join(downloadDir, authorDirName)
	logger.Infof("处理作者文件夹: %s", authorDir)
	entries, err := os.ReadDir(authorDir)
	if err != nil {
		logger.Errorf("读取 %s 文件夹失败: %v", authorDir, err)
		return
	}

	// 收集版本
	versions := []string{}
	for _, e := range entries {
		if e.IsDir() && versionRe.MatchString(e.Name()) {
			versions = append(versions, normalizeVersion(e.Name()))
		}
		// fallback：从已创建的 慕容孤晴-xxx 目录读取版本
		if e.IsDir() && strings.HasPrefix(e.Name(), authorDirName+"-") {
			ver := strings.TrimPrefix(e.Name(), authorDirName+"-")
			if !contains(versions, ver) {
				versions = append(versions, ver)
			}
		}
	}
	logger.Infof("  发现版本: %v", versions)

	// 创建版本文件夹
	for _, ver := range versions {
		os.MkdirAll(filepath.Join(authorDir, authorDirName+"-"+ver, CatPlant), os.ModePerm)
	}

	// 处理每个版本目录
	for _, verEntry := range entries {
		if !verEntry.IsDir() || !versionRe.MatchString(verEntry.Name()) {
			continue
		}
		ver := normalizeVersion(verEntry.Name())
		verPath := filepath.Join(authorDir, verEntry.Name())

		for _, catEntry := range mustReadDir(verPath) {
			// 散落文件 → 其他/
			if !catEntry.IsDir() {
				otherDir := filepath.Join(authorDir, CatOther)
				os.MkdirAll(otherDir, os.ModePerm)
				os.Rename(filepath.Join(verPath, catEntry.Name()), filepath.Join(otherDir, catEntry.Name()))
				logger.Infof("  %s/%s → 其他/", verEntry.Name(), catEntry.Name())
				continue
			}

			catName := catEntry.Name()
			// 分类目录：BepInEx二创植物 → 植物MOD
			if strings.Contains(catName, "植物") {
				logger.Infof("  %s/%s → %s/植物MOD", verEntry.Name(), catName, ver)
				// 解压每个 zip 提取 dll
				for _, zipEntry := range mustReadDir(filepath.Join(verPath, catName)) {
					if !zipEntry.IsDir() && strings.HasSuffix(strings.ToLower(zipEntry.Name()), ".zip") {
						zipPath := filepath.Join(verPath, catName, zipEntry.Name())
						modName := parseChineseName(strings.TrimSuffix(zipEntry.Name(), filepath.Ext(zipEntry.Name())))
						dlls := extractDllsFromZip(zipPath)
						if len(dlls) == 0 {
							logger.Warnf("    未找到 dll: %s", zipEntry.Name())
							continue
						}
						destDir := filepath.Join(authorDir, authorDirName+"-"+ver, CatPlant, modName)
						os.MkdirAll(destDir, os.ModePerm)
						for _, dll := range dlls {
							dst := filepath.Join(destDir, filepath.Base(dll))
							if err := copyFile(dll, dst); err != nil {
								logger.Warnf("    复制失败: %s", filepath.Base(dll))
							} else {
								logger.Infof("    → %s/植物MOD/%s/%s", ver, modName, filepath.Base(dll))
							}
						}
					}
				}
			} else {
				// 未识别的分类 → 其他/
				otherDir := filepath.Join(authorDir, CatOther)
				os.MkdirAll(otherDir, os.ModePerm)
				moveAll(filepath.Join(verPath, catName), otherDir)
				logger.Infof("  %s/%s → 其他/", verEntry.Name(), catName)
			}
		}
	}

	// 清理原始版本目录
	for _, e := range entries {
		if e.IsDir() && versionRe.MatchString(e.Name()) {
			os.RemoveAll(filepath.Join(authorDir, e.Name()))
			logger.Infof("  [清理] %s/", e.Name())
		}
	}
}

// HandleTheLinQiuFolder 处理林秋鲑鱼文件夹
// 结构：鲑鱼MOD整理 3.7 (6.19更新）.zip → BepinEX版本/2.Mod植物/CatShroom-猫娘胆小菇/BepInEx/plugins/xxx.dll
func HandleTheLinQiuFolder(downloadDir string, authorDirName string) {
	authorDir := filepath.Join(downloadDir, authorDirName)
	logger.Infof("处理作者文件夹: %s", authorDir)
	entries, err := os.ReadDir(authorDir)
	if err != nil {
		logger.Errorf("读取 %s 文件夹失败: %v", authorDir, err)
		return
	}

	// 散落文件/文件夹 → 其他/（跳过已创建的版本文件夹和目标文件夹）
	otherDir := filepath.Join(authorDir, CatOther)
	os.MkdirAll(otherDir, os.ModePerm)
	for _, e := range entries {
		name := e.Name()
		// 跳过 zip 文件、目标分类文件夹、已创建的版本文件夹
		if strings.HasSuffix(strings.ToLower(name), ".zip") {
			continue
		}
		if name == CatOther || strings.HasPrefix(name, authorDirName+"-") {
			continue
		}
		src := filepath.Join(authorDir, name)
		dst := filepath.Join(otherDir, name)
		os.Rename(src, dst)
		logger.Infof("  %s → 其他/", name)
	}

	// fallback：如果没有 zip，从已创建的 林秋鲑鱼-xxx 目录读取版本
	hasZip := false
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			hasZip = true
			break
		}
	}
	if !hasZip {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), authorDirName+"-") {
				logger.Infof("  跳过已存在: %s", e.Name())
			}
		}
		return
	}

	// 处理每个 zip
	for _, e := range entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		// 提取版本号：鲑鱼MOD整理 3.6.1 (5.31更新）.zip → 3.6.1
		ver := extractVersionFromName(e.Name())
		logger.Infof("  解压: %s (版本: %s)", e.Name(), ver)

		zipPath := filepath.Join(authorDir, e.Name())
		tmpDir := filepath.Join(authorDir, "_tmp_linQiu")
		os.MkdirAll(tmpDir, os.ModePerm)
		if err := Unzip(zipPath, tmpDir); err != nil {
			logger.Warnf("  解压失败: %s", e.Name())
			os.RemoveAll(tmpDir)
			continue
		}

		// 查找 BepinEX版本 目录
		bepinexDir := ""
		filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && strings.Contains(strings.ToLower(info.Name()), "bepin") {
				bepinexDir = path
				return filepath.SkipAll
			}
			return nil
		})
		if bepinexDir == "" {
			logger.Warnf("  未找到 BepinEX版本 目录")
			os.RemoveAll(tmpDir)
			continue
		}

		// 创建版本文件夹
		verDir := authorDirName + "-" + ver
		os.MkdirAll(filepath.Join(authorDir, verDir, CatPlant), os.ModePerm)
		os.MkdirAll(filepath.Join(authorDir, verDir, CatZombie), os.ModePerm)
		os.MkdirAll(filepath.Join(authorDir, verDir, CatSkin), os.ModePerm)

		// 遍历分类目录
		for _, catEntry := range mustReadDir(bepinexDir) {
			if !catEntry.IsDir() {
				continue
			}
			catName := catEntry.Name()

			// 跳过
			if strings.Contains(catName, "AllMod") || strings.Contains(catName, "前置") || strings.Contains(catName, "框架") {
				continue
			}

			// 关卡、插件 → 其他/
			if strings.Contains(catName, "关卡") || strings.Contains(catName, "插件") {
				moveAll(filepath.Join(bepinexDir, catName), otherDir)
				logger.Infof("    %s → 其他/", catName)
				continue
			}

			// 植物、僵尸、皮肤
			cat := ""
			if strings.Contains(catName, "植物") {
				cat = CatPlant
			} else if strings.Contains(catName, "僵尸") {
				cat = CatZombie
			} else if strings.Contains(catName, "皮肤") || strings.Contains(catName, "Skin") {
				cat = CatSkin
			}
			if cat == "" {
				moveAll(filepath.Join(bepinexDir, catName), otherDir)
				logger.Infof("    %s → 其他/", catName)
				continue
			}

			// 遍历 mod 子目录
			catPath := filepath.Join(bepinexDir, catName)
			for _, modEntry := range mustReadDir(catPath) {
				if !modEntry.IsDir() {
					continue
				}
				// 跳过 All xxx集合
				if strings.Contains(modEntry.Name(), "All") && strings.Contains(modEntry.Name(), "集合") {
					continue
				}

				modPath := filepath.Join(catPath, modEntry.Name())
				modName := modEntry.Name()
				if cat != CatSkin {
					modName = parseChineseName(modEntry.Name())
				}

				if cat == CatSkin {
					// 皮肤：复制 BepInEx/plugins/skin/ 下的文件
					skinSrcDir := filepath.Join(modPath, "BepInEx", "plugins", "skin")
					if _, err := os.Stat(skinSrcDir); os.IsNotExist(err) {
						skinSrcDir = filepath.Join(modPath, "plugins", "skin")
					}
					if _, err := os.Stat(skinSrcDir); os.IsNotExist(err) {
						continue
					}
					destModDir := filepath.Join(authorDir, verDir, CatSkin, modName)
					os.MkdirAll(destModDir, os.ModePerm)
					for _, sf := range mustReadDir(skinSrcDir) {
						src := filepath.Join(skinSrcDir, sf.Name())
						dst := filepath.Join(destModDir, sf.Name())
						if sf.IsDir() {
							CopyDir(src, dst)
						} else {
							copyFile(src, dst)
						}
						logger.Infof("    → %s/%s/%s/%s", verDir, CatSkin, modName, sf.Name())
					}
				} else {
					// 植物、僵尸：提取 dll
					dlls := findModDlls(filepath.Join(modPath, "BepInEx", "plugins"))
					if len(dlls) == 0 {
						dlls = findModDlls(filepath.Join(modPath, "plugins"))
					}
					if len(dlls) == 0 {
						dlls = findModDlls(modPath)
					}
					if len(dlls) == 0 {
						continue
					}
					destModDir := filepath.Join(authorDir, verDir, cat, modName)
					os.MkdirAll(destModDir, os.ModePerm)
					for _, dll := range dlls {
						dst := filepath.Join(destModDir, filepath.Base(dll))
						if err := copyFile(dll, dst); err != nil {
							logger.Warnf("    复制失败: %s", filepath.Base(dll))
						} else {
							logger.Infof("    → %s/%s/%s/%s", verDir, cat, modName, filepath.Base(dll))
						}
					}
				}
			}
		}

		os.RemoveAll(tmpDir)
	}

	// 清理解压后的 zip
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			os.Remove(filepath.Join(authorDir, e.Name()))
			logger.Infof("  [清理] %s", e.Name())
		}
	}
}

// HandleTheWuXuanFolder 处理梧萱梦汐文件夹
// 结构：3.6.1/MOD植物/xxx/xxx.dll, 3.6.1/MOD皮肤/xxx.rar
// 目标：梧萱梦汐-版本/植物MOD/, 梧萱梦汐/皮肤/, 梧萱梦汐/插件/, 梧萱梦汐/其他/
func HandleTheWuXuanFolder(downloadDir string, authorDirName string) {
	authorDir := filepath.Join(downloadDir, authorDirName)
	logger.Infof("处理作者文件夹: %s", authorDir)
	entries, err := os.ReadDir(authorDir)
	if err != nil {
		logger.Errorf("读取 %s 文件夹失败: %v", authorDir, err)
		return
	}

	// 收集版本
	versions := []string{}
	for _, e := range entries {
		if e.IsDir() && versionRe.MatchString(e.Name()) {
			versions = append(versions, normalizeVersion(e.Name()))
		}
		// fallback：从已创建的 梧萱梦汐-xxx 目录读取版本
		if e.IsDir() && strings.HasPrefix(e.Name(), authorDirName+"-") {
			ver := strings.TrimPrefix(e.Name(), authorDirName+"-")
			if !contains(versions, ver) {
				versions = append(versions, ver)
			}
		}
	}
	logger.Infof("  发现版本: %v", versions)

	// 创建目标文件夹
	for _, ver := range versions {
		os.MkdirAll(filepath.Join(authorDir, authorDirName+"-"+ver, CatPlant), os.ModePerm)
		os.MkdirAll(filepath.Join(authorDir, authorDirName+"-"+ver, CatZombie), os.ModePerm)
		os.MkdirAll(filepath.Join(authorDir, authorDirName+"-"+ver, CatTrainer), os.ModePerm)
	}
	os.MkdirAll(filepath.Join(authorDir, CatSkin), os.ModePerm)
	os.MkdirAll(filepath.Join(authorDir, CatPlugin), os.ModePerm)
	os.MkdirAll(filepath.Join(authorDir, CatOther), os.ModePerm)

	// 白名单：已创建的目标文件夹
	keep := map[string]bool{CatSkin: true, CatPlugin: true, CatOther: true}
	for _, ver := range versions {
		keep[authorDirName+"-"+ver] = true
	}

	// 处理每个目录
	for _, entry := range entries {
		if !entry.IsDir() {
			// 散落文件 → 其他/
			os.Rename(filepath.Join(authorDir, entry.Name()), filepath.Join(authorDir, CatOther, entry.Name()))
			logger.Infof("  %s → 其他/", entry.Name())
			continue
		}

		dirName := entry.Name()

		// 跳过已创建的目标文件夹
		if keep[dirName] {
			logger.Infof("  [保留] %s", dirName)
			continue
		}

		// 版本目录（3.6.1, 3.7）
		if versionRe.MatchString(dirName) {
			ver := normalizeVersion(dirName)
			verPath := filepath.Join(authorDir, dirName)

			for _, catEntry := range mustReadDir(verPath) {
				if !catEntry.IsDir() {
					continue
				}
				catName := catEntry.Name()
				catPath := filepath.Join(verPath, catName)

				switch {
				case strings.Contains(catName, "植物"):
					dst := filepath.Join(authorDir, authorDirName+"-"+ver, CatPlant)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → %s/植物MOD", dirName, catName, ver)

				case strings.Contains(catName, "僵尸"):
					dst := filepath.Join(authorDir, authorDirName+"-"+ver, CatZombie)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → %s/僵尸MOD", dirName, catName, ver)

				case strings.Contains(catName, "修改器"):
					dst := filepath.Join(authorDir, authorDirName+"-"+ver, CatTrainer)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → %s/修改器", dirName, catName, ver)

				case strings.Contains(catName, "皮肤"):
					dst := filepath.Join(authorDir, CatSkin)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → 皮肤", dirName, catName)

				case strings.Contains(catName, "迷你") || strings.Contains(catName, "宠物") ||
					strings.Contains(catName, "手套") || strings.Contains(catName, "铲子") ||
					strings.Contains(catName, "插件"):
					dst := filepath.Join(authorDir, CatPlugin)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → 插件", dirName, catName)

				default:
					dst := filepath.Join(authorDir, CatOther)
					moveAll(catPath, dst)
					logger.Infof("  %s/%s → 其他", dirName, catName)
				}
			}
		} else {
			// 非版本目录（8.其他插件补丁...）→ 其他/
			dst := filepath.Join(authorDir, CatOther)
			moveAll(filepath.Join(authorDir, dirName), dst)
			logger.Infof("  %s → 其他", dirName)
		}
	}

	// 清理：只删原始版本目录和非版本目录，保留目标文件夹
	for _, e := range entries {
		if e.IsDir() && !keep[e.Name()] {
			os.RemoveAll(filepath.Join(authorDir, e.Name()))
			logger.Infof("  [清理] %s/", e.Name())
		}
	}
}

// extractVersionFromName 从文件名提取版本号
// "鲑鱼MOD整理 3.6.1 (5.31更新）" → "3.6.1"
func extractVersionFromName(name string) string {
	if m := zipVersionRe.FindStringSubmatch(name); len(m) > 1 {
		return m[1]
	}
	return "未分类"
}

// extractDllsFromZip 解压 zip 到临时目录，返回所有非 CustomizeLib 的 dll 路径
func extractDllsFromZip(zipPath string) []string {
	tmpDir := filepath.Join(filepath.Dir(zipPath), "_tmp")
	os.MkdirAll(tmpDir, os.ModePerm)
	if err := Unzip(zipPath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil
	}

	// 查找 dll
	dlls := findModDlls(filepath.Join(tmpDir, "BepInEx", "plugins"))
	if len(dlls) == 0 {
		dlls = findModDlls(filepath.Join(tmpDir, "plugins"))
	}
	if len(dlls) == 0 {
		dlls = findModDlls(tmpDir)
	}

	// 复制到安全位置（因为 tmpDir 会被删除）
	result := []string{}
	safeDir := filepath.Join(filepath.Dir(zipPath), "_dlls")
	os.MkdirAll(safeDir, os.ModePerm)
	for _, dll := range dlls {
		dst := filepath.Join(safeDir, filepath.Base(dll))
		if err := copyFile(dll, dst); err == nil {
			result = append(result, dst)
		}
	}
	os.RemoveAll(tmpDir)
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func mustReadDir(dir string) []os.DirEntry {
	entries, _ := os.ReadDir(dir)
	return entries
}

func parseChineseName(name string) string {
	// 取最后一个 - 后面的部分（中文名）
	// "僵尸-CannonGargantuar-伽农炮僵尸" → "伽农炮僵尸"
	// "HaavkAutoChariot-哈夫克自动战车" → "哈夫克自动战车"
	// "SolarCabbageSkinLoader-太阳神皮肤" → "太阳神皮肤"
	if idx := strings.LastIndex(name, "-"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// moveTrainerZips 只移动 PvZRHModfiedFor*.zip 到目标
func moveTrainerZips(srcDir, dstDir string) {
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		name := info.Name()
		if strings.HasPrefix(name, "PvZRHModfiedFor") && strings.HasSuffix(strings.ToLower(name), ".zip") {
			dst := filepath.Join(dstDir, name)
			if err := os.Rename(path, dst); err != nil {
				logger.Warnf("    移动失败: %s", name)
			} else {
				logger.Infof("      → %s", name)
			}
		}
		return nil
	})
}

// extractSkinZipsToFolder 解压皮肤 zip，提取 SkinLoader/数字ID/皮肤文件夹 到目标
// zip 解压后结构：xxxSkinLoader-中文名/SkinLoader/934/SolarCabbage
// 目标：dstDir/中文名/SolarCabbage
func extractSkinZipsToFolder(srcDir, dstDir string) {
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		zipPath := filepath.Join(srcDir, e.Name())
		skinName := parseChineseName(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))

		// 解压到临时目录
		tmpDir := filepath.Join(srcDir, "_tmp_skin")
		os.MkdirAll(tmpDir, os.ModePerm)
		if err := Unzip(zipPath, tmpDir); err != nil {
			logger.Warnf("    解压失败: %s", e.Name())
			os.RemoveAll(tmpDir)
			continue
		}

		// 查找 SkinLoader 目录（可能在根目录或嵌套子目录下）
		skinLoaderDir := ""
		filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == "SkinLoader" {
				skinLoaderDir = path
				return filepath.SkipAll
			}
			return nil
		})

		if skinLoaderDir == "" {
			logger.Warnf("    未找到 SkinLoader: %s", e.Name())
			os.RemoveAll(tmpDir)
			continue
		}

		// 遍历 SkinLoader/数字ID/ 下的皮肤文件或文件夹
		for _, idDir := range mustReadDir(skinLoaderDir) {
			if !idDir.IsDir() {
				continue
			}
			for _, sf := range mustReadDir(filepath.Join(skinLoaderDir, idDir.Name())) {
				srcSkin := filepath.Join(skinLoaderDir, idDir.Name(), sf.Name())
				destSkin := filepath.Join(dstDir, skinName, sf.Name())
				os.MkdirAll(filepath.Dir(destSkin), os.ModePerm)
				if sf.IsDir() {
					if err := CopyDir(srcSkin, destSkin); err != nil {
						logger.Warnf("    复制皮肤失败: %s", sf.Name())
					} else {
						logger.Infof("    → 皮肤/%s/%s/", skinName, sf.Name())
					}
				} else {
					if err := copyFile(srcSkin, destSkin); err != nil {
						logger.Warnf("    复制皮肤失败: %s", sf.Name())
					} else {
						logger.Infof("    → 皮肤/%s/%s", skinName, sf.Name())
					}
				}
			}
		}
		os.RemoveAll(tmpDir)
	}
}

// extractZipsToFolder 解压目录下所有 zip，提取 dll 到目标文件夹
func extractZipsToFolder(srcDir, dstDir string) {
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		zipPath := filepath.Join(srcDir, e.Name())
		modName := parseChineseName(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		dlls := extractDllsFromZip(zipPath)
		if len(dlls) == 0 {
			logger.Warnf("    未找到 dll: %s", e.Name())
			continue
		}
		destModDir := filepath.Join(dstDir, modName)
		os.MkdirAll(destModDir, os.ModePerm)
		for _, dll := range dlls {
			dst := filepath.Join(destModDir, filepath.Base(dll))
			if err := copyFile(dll, dst); err != nil {
				logger.Warnf("    复制失败: %s", filepath.Base(dll))
			} else {
				logger.Infof("    → %s/%s/%s", filepath.Base(dstDir), modName, filepath.Base(dll))
			}
		}
		// 清理解压临时文件
		os.RemoveAll(filepath.Join(srcDir, "_dlls"))
		os.RemoveAll(filepath.Join(srcDir, "_tmp"))
	}
}

// moveAll 移动目录下所有内容到目标
func moveAll(srcDir, dstDir string) {
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			logger.Warnf("    移动失败: %s, %v", e.Name(), err)
		}
	}
}

// classifyGaoShuCategory 分类高数羽衫的目录名
func classifyGaoShuCategory(name string) string {
	if name == "修改器" || name == "1.修改器" {
		return CatTrainer
	}
	if name == "植物皮肤" {
		return CatSkin
	}
	if name == "二创僵尸" || name == "3.二创僵尸" {
		return CatZombie
	}
	if name == "二创植物" || name == "2.二创植物" {
		return CatPlant
	}
	if name == "二创小宠物" || name == "4.二创小宠物" {
		return CatPet
	}
	if name == "融合Mod" {
		return CatOther
	}
	if name == "周年庆单品" {
		return CatPlant
	}
	return ""
}

func GetGaoShuModVersion(authorDir string) []string {
	seen := map[string]bool{}
	var versions []string

	entries, err := os.ReadDir(authorDir)
	if err != nil {
		return nil
	}

	// 方式1：从原始分类子目录读取版本（1.修改器/3.6.1 等）
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subEntries, _ := os.ReadDir(filepath.Join(authorDir, entry.Name()))
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			if versionRe.MatchString(sub.Name()) {
				ver := normalizeVersion(sub.Name())
				if !seen[ver] {
					seen[ver] = true
					versions = append(versions, ver)
				}
			}
		}
	}

	// 方式2：从已有的 高数羽衫-* 目录读取版本
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 高数羽衫-3.6.1 → 3.6.1
		if strings.HasPrefix(name, "高数羽衫-") {
			ver := strings.TrimPrefix(name, "高数羽衫-")
			if !seen[ver] {
				seen[ver] = true
				versions = append(versions, ver)
			}
		}
	}

	return versions
}

func normalizeVersion(name string) string {
	if m := compatRe.FindStringSubmatch(name); len(m) > 1 {
		return m[1]
	}
	return name
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// findModDlls 查找目录下所有非 CustomizeLib 的 dll
func findModDlls(dir string) []string {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var dlls []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".dll") {
			continue
		}
		if strings.EqualFold(name, "CustomizeLib.BepInEx.dll") || strings.EqualFold(name, "CustomizeLib.dll") {
			continue
		}
		dlls = append(dlls, filepath.Join(dir, name))
	}
	return dlls
}

// generateMergedModJSON 扫描所有作者目录，生成合并的 mod.json
func generateMergedModJSON(downloadDir string) {
	var allMods []ModJSON

	authorDirs := mustReadDir(downloadDir)
	for _, authorEntry := range authorDirs {
		if !authorEntry.IsDir() {
			continue
		}
		authorName := authorEntry.Name()
		authorDir := filepath.Join(downloadDir, authorName)

		// 查找版本目录（作者名-版本）
		for _, verEntry := range mustReadDir(authorDir) {
			if !verEntry.IsDir() {
				continue
			}
			verName := verEntry.Name()
			// 只处理 作者-版本 格式的目录
			if !strings.HasPrefix(verName, authorName+"-") {
				continue
			}
			ver := strings.TrimPrefix(verName, authorName+"-")
			verDir := filepath.Join(authorDir, verName)

			mj := ModJSON{
				Author:  authorName,
				Version: ver,
			}

			// 遍历分类目录
			for _, catEntry := range mustReadDir(verDir) {
				if !catEntry.IsDir() {
					continue
				}
				catPath := filepath.Join(verDir, catEntry.Name())

				// 遍历 mod 子目录
				for _, modEntry := range mustReadDir(catPath) {
					if !modEntry.IsDir() {
						continue
					}
					item := ModJSONItem{
						ChineseName: modEntry.Name(),
					}

					// 收集 dll 名称
					modPath := filepath.Join(catPath, modEntry.Name())
					dlls := findModDlls(modPath)
					modNames := []string{}
					for _, dll := range dlls {
						modNames = append(modNames, filepath.Base(dll))
					}
					item.ModName = strings.Join(modNames, ", ")

					// 按分类添加到对应列表
					switch catEntry.Name() {
					case CatPlant:
						mj.PlanList = append(mj.PlanList, item)
					case CatZombie:
						mj.ZombieList = append(mj.ZombieList, item)
					case CatSkin:
						mj.SkinList = append(mj.SkinList, item)
					case CatPlugin, CatTrainer, CatPet:
						mj.PluginList = append(mj.PluginList, item)
					}
				}
			}

			if len(mj.PlanList) > 0 || len(mj.ZombieList) > 0 || len(mj.SkinList) > 0 || len(mj.PluginList) > 0 {
				allMods = append(allMods, mj)
			}
		}
	}

	// 写入 mod.json
	outputPath := filepath.Join(downloadDir, "mod.json")
	data, err := json.MarshalIndent(allMods, "", "  ")
	if err != nil {
		logger.Errorf("序列化 mod.json 失败: %v", err)
		return
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		logger.Errorf("写入 mod.json 失败: %v", err)
		return
	}
	logger.Infof("已生成 mod.json: %s (%d 条记录)", outputPath, len(allMods))
}
