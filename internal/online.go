package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pveRH-mod-manager/internal/logger"
)

// OnlineMod 在线 Mod 信息
type OnlineMod struct {
	GameVer  string `json:"game_ver"`  // 游戏版本
	Author   string `json:"author"`    // 作者
	ModType  string `json:"mod_type"`  // Mod 类型（可为空）
	NameCN   string `json:"name_cn"`   // 中文名称
	NameEN   string `json:"name_en"`   // 英文名称（不含扩展名）
	FileName string `json:"file_name"` // 完整文件名，如 "xxx.dll"
	URL      string `json:"url"`       // 下载直链
}

// FetchGameVersions 从服务器获取可用的游戏版本列表
func FetchGameVersions(serverURL string) ([]string, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/versions"
	logger.Debugf("请求游戏版本列表: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		logger.Errorf("获取版本列表失败: %v", err)
		return nil, fmt.Errorf("无法获取版本列表: %v", err)
	}
	logger.Debugf("版本列表响应: status=%d", resp.StatusCode)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	var versions []string
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("解析版本列表失败: %v", err)
	}
	return versions, nil
}

// FetchOnlineMods 根据游戏版本从服务器获取在线 Mod 列表
func FetchOnlineMods(serverURL, gameVersion string) ([]OnlineMod, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/mods"
	if gameVersion != "" {
		url += "?ver=" + gameVersion
	}
	logger.Debugf("请求在线 Mod 列表: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		logger.Errorf("获取在线 Mod 列表失败: %v", err)
		return nil, fmt.Errorf("无法获取在线 Mod: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	var mods []OnlineMod
	if err := json.NewDecoder(resp.Body).Decode(&mods); err != nil {
		return nil, fmt.Errorf("解析 Mod 列表失败: %v", err)
	}
	return mods, nil
}

// DownloadMod 下载单个 Mod 并保存到 Mod 库对应路径
// 保存路径格式：{游戏版本}/{作者}/[Mod类型]/{中文名}/{文件名}
func DownloadMod(mod OnlineMod, modLibPath, serverURL string) error {
	logger.Infof("开始下载 Mod: %s/%s, 版本: %s, 作者: %s", mod.NameCN, mod.FileName, mod.GameVer, mod.Author)
	// 构建保存目录
	saveDir := filepath.Join(modLibPath, mod.GameVer, mod.Author)
	logger.Debugf("构建保存目录: %s", saveDir)
	if mod.ModType != "" {
		saveDir = filepath.Join(saveDir, mod.ModType)
	}
	saveDir = filepath.Join(saveDir, mod.NameCN)
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	savePath := filepath.Join(saveDir, mod.FileName)

	// 构建下载链接
	downloadURL := mod.URL
	if downloadURL == "" {
		// 如果服务器未提供直链，根据规则拼接（假设文件可通过 /files/ 路径访问）
		downloadURL = strings.TrimRight(serverURL, "/") + "/files/" + mod.FileName
	}
	logger.Debugf("下载链接: %s", downloadURL)

	// 执行下载
	resp, err := http.Get(downloadURL)
	if err != nil {
		logger.Errorf("下载请求失败: %s, %v", downloadURL, err)
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	outFile, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		logger.Errorf("写入文件失败: %s, %v", savePath, err)
		return fmt.Errorf("写入文件失败: %v", err)
	}

	logger.Infof("下载完成: %s", savePath)
	return nil
}
