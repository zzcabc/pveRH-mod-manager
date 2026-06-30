package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ===== 服务器返回模型 =====

// ServerModInfo 服务器返回的 MOD 信息
type ServerModInfo struct {
	GameVer  string `json:"game_ver"`
	Author   string `json:"author"`
	ModType  string `json:"mod_type"`
	NameCN   string `json:"name_cn"`
	NameEN   string `json:"name_en"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

// ===== HTTP 客户端 =====

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

// ===== API 调用 =====

// FetchVersions 从服务器获取可用版本列表
func FetchVersions(serverURL string) ([]string, error) {
	apiURL := fmt.Sprintf("%s/api/versions", strings.TrimRight(serverURL, "/"))

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求版本列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误: %d", resp.StatusCode)
	}

	var versions []string
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("解析版本列表失败: %w", err)
	}

	return versions, nil
}

// FetchAuthors 从服务器获取作者列表
func FetchAuthors(serverURL string) ([]string, error) {
	apiURL := fmt.Sprintf("%s/api/authors", strings.TrimRight(serverURL, "/"))

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求作者列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误: %d", resp.StatusCode)
	}

	var authors []string
	if err := json.NewDecoder(resp.Body).Decode(&authors); err != nil {
		return nil, fmt.Errorf("解析作者列表失败: %w", err)
	}

	return authors, nil
}

// FetchMods 从服务器查询 MOD 列表
// ver/author/modType 为空表示不过滤
func FetchMods(serverURL, ver, author, modType string) ([]ServerModInfo, error) {
	apiURL := fmt.Sprintf("%s/api/mods", strings.TrimRight(serverURL, "/"))

	params := url.Values{}
	if ver != "" {
		params.Set("ver", ver)
	}
	if author != "" {
		params.Set("author", author)
	}
	if modType != "" {
		params.Set("type", modType)
	}
	if len(params) > 0 {
		apiURL += "?" + params.Encode()
	}

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求 MOD 列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误: %d", resp.StatusCode)
	}

	var mods []ServerModInfo
	if err := json.NewDecoder(resp.Body).Decode(&mods); err != nil {
		return nil, fmt.Errorf("解析 MOD 列表失败: %w", err)
	}

	return mods, nil
}

// DownloadModFile 通过服务器 URL 下载 MOD 文件到本地目录
// 若 downloadURL 是相对路径（以 / 开头），自动拼接 serverURL
func DownloadModFile(downloadURL, destDir, serverURL string) (string, error) {
	fullURL := downloadURL
	if strings.HasPrefix(downloadURL, "/") {
		fullURL = strings.TrimRight(serverURL, "/") + downloadURL
	}
	resp, err := httpClient.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载返回错误: %d", resp.StatusCode)
	}

	// 从 Content-Disposition 或 URL 提取文件名
	fileName := extractFileName(resp, downloadURL)
	destPath := filepath.Join(destDir, fileName)

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", fmt.Errorf("创建下载目录失败: %w", err)
	}

	// 写入文件
	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return destPath, nil
}

// DownloadAndInstallServerMod 下载在线 MOD 并安装到游戏目录
func DownloadAndInstallServerMod(info ServerModInfo, gamePath, serverURL string) error {
	pluginsDir := filepath.Join(gamePath, "BepInEx", "plugins")
	if !DirExists(pluginsDir) {
		if err := os.MkdirAll(pluginsDir, 0755); err != nil {
			return fmt.Errorf("创建 plugins 目录失败: %w", err)
		}
	}

	fmt.Printf("下载 MOD: %s (%s)\n", info.NameCN, info.Author)

	destPath, err := DownloadModFile(info.URL, pluginsDir, serverURL)
	if err != nil {
		return fmt.Errorf("下载 MOD 失败: %w", err)
	}

	fmt.Printf("已下载: %s\n", destPath)
	return nil
}

// ===== 辅助 =====

// extractFileName 从响应头或 URL 中提取文件名
func extractFileName(resp *http.Response, downloadURL string) string {
	// 优先从 Content-Disposition 获取
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		// attachment; filename="xxx.dll"
		const prefix = `filename="`
		start := strings.Index(cd, prefix)
		if start >= 0 {
			start += len(prefix)
			end := strings.Index(cd[start:], `"`)
			if end >= 0 {
				return cd[start : start+end]
			}
		}
	}

	// 从 URL 参数获取 name
	u, err := url.Parse(downloadURL)
	if err == nil {
		name := u.Query().Get("name")
		if name != "" {
			return name
		}
	}

	// 从 URL 路径末尾获取
	path := u.Path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}

	return "mod_download"
}
