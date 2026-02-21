package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/VectorBits/Vespera/src/cmd"
	"github.com/VectorBits/Vespera/src/internal/ui"
)

//go:embed config/settings.example.yaml
//go:embed strategy/prompts/mode1/*.tmpl
//go:embed strategy/prompts/mode2/*.tmpl
//go:embed strategy/exp_libs/mode1/*.toml
var embeddedFiles embed.FS

func main() {
	// 初始化默认资源文件 (配置和策略)
	if err := initResources(); err != nil {
		cmd.PrintFatal(err)
	}

	cmd.Print()
	if err := cmd.Run(); err != nil {
		cmd.PrintFatal(err)
	}
}

func initResources() error {
	// 1. 初始化配置文件
	if err := initConfigFile(); err != nil {
		return fmt.Errorf("failed to init config file: %w", err)
	}

	// 2. 初始化策略文件
	if err := initStrategyFiles(); err != nil {
		return fmt.Errorf("failed to init strategy files: %w", err)
	}

	return nil
}

func initConfigFile() error {
	targetDir := "config"
	targetFile := filepath.Join(targetDir, "settings.yaml")

	// 检查目标文件是否存在
	if _, err := os.Stat(targetFile); err == nil {
		return nil // 已存在，跳过
	}

	// 创建目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// 读取嵌入的源文件
	data, err := embeddedFiles.ReadFile("config/settings.example.yaml")
	if err != nil {
		return err
	}

	// 写入目标文件
	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return err
	}

	fmt.Printf(ui.Green+"✅ Created default config file: %s"+ui.Reset+"\n", targetFile)
	return nil
}

func initStrategyFiles() error {
	// 需要释放的目录列表
	dirs := []string{
		"strategy/prompts/mode1",
		"strategy/prompts/mode2",
		"strategy/exp_libs/mode1",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// 遍历并释放所有嵌入的策略文件
	err := fs.WalkDir(embeddedFiles, "strategy", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// 读取文件内容
		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return err
		}

		// 检查目标文件是否存在，不存在则写入
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// 确保父目录存在
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}
			fmt.Printf(ui.Green+"✅ Restored strategy file: %s"+ui.Reset+"\n", path)
		}

		return nil
	})

	return err
}
