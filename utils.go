package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists 检查目录是否存在
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// PathExists 检查路径是否存在（文件或目录）
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir 判断路径末尾是否带 / （表示目录类型条目）
func IsDir(path string) bool {
	return strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\")
}

// CopyDir 递归复制目录内容
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源不是目录: %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取目录内容失败: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 保留文件权限
	srcInfo, _ := srcFile.Stat()
	if srcInfo != nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}

// Unzip 解压 zip 文件到目标目录
func Unzip(src, dst string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("创建解压目标目录失败: %w", err)
	}

	for _, f := range reader.File {
		targetPath := filepath.Join(dst, f.Name)

		// Zip Slip 防护
		absDst, _ := filepath.Abs(dst)
		absTarget, _ := filepath.Abs(targetPath)
		if !strings.HasPrefix(absTarget, absDst) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, f.FileInfo().Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		srcFile, err := f.Open()
		if err != nil {
			return fmt.Errorf("打开 zip 内文件失败: %w", err)
		}

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("创建解压文件失败: %w", err)
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return fmt.Errorf("解压写入失败: %w", err)
		}
	}

	return nil
}

// RemovePath 删除文件或目录（目录则递归删除）
func RemovePath(path string) error {
	if !PathExists(path) {
		return nil
	}
	return os.RemoveAll(path)
}

// CleanDirName 去掉路径末尾的 / 符号，获取纯净目录名
// 如 "BepInEx/" → "BepInEx"
func CleanDirName(path string) string {
	return strings.TrimRight(path, "/\\")
}

// IsSubPath 检查 target 是否是 base 的子路径
func IsSubPath(base, target string) bool {
	absBase, _ := filepath.Abs(base)
	absTarget, _ := filepath.Abs(target)
	return strings.HasPrefix(absTarget, absBase)
}

// SelectFolder 调用系统文件夹选择对话框，返回所选路径
// 通过临时文件传递路径，避免 PowerShell 管道编码问题（GBK→UTF-8 乱码）
func SelectFolder() (string, error) {
	// 创建临时输出文件
	tmpFile, err := os.CreateTemp("", "pverh-folder-*.txt")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	defer os.Remove(tmpPath)

	// PowerShell 脚本：选择目录后写入临时文件（UTF-8）
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.FolderBrowserDialog
$f.Description = "请选择目录"
$f.ShowNewFolderButton = $true
if ($f.ShowDialog() -eq 'OK') {
    $f.SelectedPath | Out-File -FilePath '%s' -Encoding utf8 -NoNewline
}
`, strings.ReplaceAll(tmpPath, "'", "''"))

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("选择目录失败: %w", err)
	}

	// 从临时文件读取 UTF-8 路径（PowerShell 的 Out-File -Encoding utf8 带 BOM）
	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("未选择目录")
	}

	// 去掉 UTF-8 BOM (EF BB BF) 和尾部空白
	data = trimBOM(data)
	path := strings.TrimSpace(string(data))
	if path == "" {
		return "", fmt.Errorf("未选择目录")
	}

	return path, nil
}

// trimBOM 去掉 UTF-8 BOM 头部
func trimBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}
