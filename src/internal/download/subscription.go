package download

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/VectorBits/Vespera/src/internal/config"
	"github.com/VectorBits/Vespera/src/internal/ui"
)

func (d *Downloader) SubscribeNewContracts(ctx context.Context, callback func(address string)) error {
	log.Println(ui.Red + "🔴 Starting 7x24h blockchain monitoring mode..." + ui.Reset)

	startBlock, err := d.GetCurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	log.Printf(ui.Cyan+"📍 Monitoring start: Block %d (Only monitor new blocks, no history download)"+ui.Reset+"\n", startBlock)

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	lastProcessedBlock := startBlock

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			currentBlock, err := d.GetCurrentBlock(ctx)
			if err != nil {
				log.Printf(ui.Yellow+"⚠️ failed to get latest block: %v"+ui.Reset, err)
				continue
			}

			if currentBlock <= lastProcessedBlock {
				continue
			}

			log.Printf(ui.Blue+"📦 New blocks found: %d - %d"+ui.Reset+"\n", lastProcessedBlock+1, currentBlock)

			if err := d.DownloadBlockRange(ctx, lastProcessedBlock+1, currentBlock); err != nil {
				log.Printf(ui.Yellow+"⚠️ block sync failed: %v"+ui.Reset, err)
				continue
			}

			newContracts, err := d.getNewContractsAfterBlock(ctx, lastProcessedBlock)
			if err != nil {
				log.Printf(ui.Yellow+"⚠️ failed to query new contracts: %v"+ui.Reset, err)
				continue
			}

			if len(newContracts) > 0 {
				log.Printf(ui.Green+"📢 Found %d new open source contracts, pushing to scan queue..."+ui.Reset+"\n", len(newContracts))
				for _, addr := range newContracts {
					callback(addr)
				}
			} else {
				log.Printf(ui.Gray+"ℹ️ No new open source contracts in block %d-%d"+ui.Reset+"\n", lastProcessedBlock+1, currentBlock)
			}

			lastProcessedBlock = currentBlock
		}
	}
}

func (d *Downloader) getNewContractsAfterBlock(ctx context.Context, blockNum uint64) ([]string, error) {
	tableName, err := config.GetTableName(d.ChainName)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT address FROM %s WHERE createblock > ? AND isopensource = 1", tableName)
	rows, err := d.db.QueryContext(ctx, query, blockNum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addrs []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			continue
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}
