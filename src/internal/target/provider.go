package target

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/VectorBits/Vespera/src/internal"
	"github.com/VectorBits/Vespera/src/internal/config"
	"github.com/VectorBits/Vespera/src/internal/dbutil"
	"github.com/VectorBits/Vespera/src/internal/download"
	"github.com/VectorBits/Vespera/src/internal/logger"
)

// helloq GetTargetChannel 目标地址生产者
func GetTargetChannel(ctx context.Context, cfg config.ScanConfiguration, downloader *download.Downloader, db *sql.DB) (<-chan string, error) {
	// 创建缓冲通道
	out := make(chan string, 100)

	// 启动 Producer Goroutine
	go func() {
		defer close(out) // 任务结束时关闭通道

		// 处理 -t last 持续监控模式
		if strings.ToLower(cfg.TargetSource) == "last" {
			if downloader == nil {
				logger.Error("Monitor mode requires valid downloader")
				return
			}
			logger.Info("Starting 7x24h monitor mode (-t last)...")

			// 订阅新合约事件 (阻塞调用，直到 ctx 取消)
			err := downloader.SubscribeNewContracts(ctx, func(newAddr string) {
				select {
				case out <- newAddr:
					// 成功发送
				case <-ctx.Done():
					// 上下文取消，停止发送
				}
			})
			if err != nil {
				logger.Error("Monitor interrupted: %v", err)
			}
			return
		}

		// 处理静态目标模式 (File, DB, Single Address)
		// 使用 helper 函数解析所有目标
		targets, err := resolveStaticTargets(cfg, db)
		if err != nil {
			logger.Error("Failed to resolve targets: %v", err)
			return
		}

		logger.Info("Loaded %d static targets", len(targets))
		for _, addr := range targets {
			select {
			case out <- addr:
				// 成功发送
			case <-ctx.Done():
				// 上下文取消，提前退出
				return
			}
		}
	}()

	return out, nil
}

func resolveStaticTargets(cfg config.ScanConfiguration, db *sql.DB) ([]string, error) {
	switch strings.ToLower(cfg.TargetSource) {
	case "db":
		// 使用统一的 dbutil 获取地址，避免循环引用且保证逻辑一致
		return dbutil.GetAddressesFromDB(db, cfg.Chain, cfg.BlockRange)

	case "file", "filepath":
		// 同上，需要复用读取文件的逻辑
		return internal.ReadLines(cfg.TargetFile)

	case "contract", "address", "single":
		if strings.TrimSpace(cfg.TargetAddress) == "" {
			return nil, fmt.Errorf("missing target address: -addr")
		}
		return []string{strings.TrimSpace(cfg.TargetAddress)}, nil

	default:
		return nil, fmt.Errorf("unsupported target source: %s", cfg.TargetSource)
	}
}
