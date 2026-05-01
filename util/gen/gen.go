package gen

import (
	"fmt"
	"os"
	"path/filepath"
)

// OutDir 根据 path 生成绝对路径并检查该路径是否存在。
// 成功时返回带有末尾 '/' 的绝对路径，若路径不存在则返回错误。
func OutDir(path string) (string, error) {
	outDir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(outDir)
	if err != nil {
		return "", err
	}

	if !stat.IsDir() {
		return "", fmt.Errorf("output directory %s is not a directory", outDir)
	}
	outDir = outDir + "/"
	return outDir, nil
}
