package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Storage interface {
	Save(report *Report, content string) (string, error)
}

type FileStorage struct {
	OutputDir string
}

func NewFileStorage(outputDir string) *FileStorage {
	return &FileStorage{
		OutputDir: outputDir,
	}
}

func sanitizeFilenameComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := b.String()
	out = strings.Trim(out, "._-")
	if out == "" {
		return "unknown"
	}
	return out
}

func (s *FileStorage) Save(report *Report, content string) (string, error) {
	if s.OutputDir == "" {
		s.OutputDir = "reports"
	}
	// 确保输出目录存在
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// 生成文件名
	timestamp := time.Now().UnixNano()
	mode := sanitizeFilenameComponent(report.Mode)
	filename := fmt.Sprintf("scan_report_%s_%d.md", mode, timestamp)
	reportPath := filepath.Join(s.OutputDir, filename)

	tmpFile, err := os.CreateTemp(s.OutputDir, filename+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp report file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write temp report file: %w", err)
	}
	if err := tmpFile.Chmod(0644); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to chmod temp report file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp report file: %w", err)
	}

	if err := os.Rename(tmpPath, reportPath); err != nil {
		return "", fmt.Errorf("failed to finalize report file: %w", err)
	}

	return reportPath, nil
}
