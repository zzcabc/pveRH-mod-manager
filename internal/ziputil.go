package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode/v2"

	"pveRH-mod-manager/internal/logger"
)

// Unzip 解压 ZIP 文件到目标目录
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		logger.Errorf("打开 ZIP 失败: %s, %v", src, err)
		return err
	}
	defer r.Close()

	logger.Debugf("开始解压 ZIP: %s -> %s, 共 %d 个文件", src, dest, len(r.File))

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("非法文件路径: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer outFile.Close()

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}
	logger.Debugf("ZIP 解压完成: %s", src)
	return nil
}

// Unrar 解压 RAR 文件到目标目录
func Unrar(src, dest string) error {
	rr, err := rardecode.OpenReader(src)
	if err != nil {
		logger.Errorf("打开 RAR 失败: %s, %v", src, err)
		return err
	}
	defer rr.Close()

	logger.Debugf("开始解压 RAR: %s -> %s", src, dest)

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Errorf("RAR 读取条目失败: %v", err)
			return err
		}

		fpath := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", fpath)
		}

		if header.IsDir {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode()))
		if err != nil {
			return err
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, rr)
		if err != nil {
			return err
		}
	}
	logger.Debugf("RAR 解压完成: %s", src)
	return nil
}

// CopyDir 递归复制目录
func CopyDir(src, dst string) error {
	logger.Debugf("开始复制目录: %s -> %s", src, dst)
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
	if err != nil {
		logger.Errorf("复制目录失败: %s -> %s, %v", src, dst, err)
		return err
	}
	logger.Debugf("目录复制完成: %s", src)
	return nil
}

// GetZipTopDir 返回 ZIP 内顶层文件夹名称
func GetZipTopDir(zipPath string) (string, error) {
	logger.Debugf("读取 ZIP 顶层目录: %s", zipPath)
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	topDirs := make(map[string]struct{})
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if parts[0] != "" {
			topDirs[parts[0]] = struct{}{}
		}
	}
	for dir := range topDirs {
		logger.Debugf("ZIP 顶层目录: %s", dir)
		return dir, nil
	}
	return "", fmt.Errorf("zip 内没有顶层文件夹")
}
