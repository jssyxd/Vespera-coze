package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const Clear = "\033[2K\r"

type ProgressBar struct {
	total       int
	current     int
	vulnCount   int
	startTime   time.Time
	description string
	mu          sync.Mutex
	width       int
}

func NewProgressBar(total int, description string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		current:     0,
		startTime:   time.Now(),
		description: description,
		width:       40, // 进度条长度
	}
}

func (pb *ProgressBar) Increment() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.current++
	pb.render()
}

func (pb *ProgressBar) AddVuln() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.vulnCount++
	// 不需要重新渲染，下次 Update 或 Increment 会更新
}

func (pb *ProgressBar) PrintMsg(msg string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	// 清除当前行（进度条），打印消息，然后换行，最后重绘进度条
	fmt.Print(Clear)
	fmt.Println(msg)
	pb.render()
}

func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	// 确保进度满格
	pb.current = pb.total
	fmt.Print(Clear)
	pb.render()
	fmt.Println() // 换行
}

func (pb *ProgressBar) render() {
	percent := float64(pb.current) / float64(pb.total)
	if percent > 1.0 {
		percent = 1.0
	}

	filled := int(float64(pb.width) * percent)
	bar := strings.Repeat("=", filled)
	if filled < pb.width {
		bar += ">" + strings.Repeat(".", pb.width-filled-1)
	} else {
		// 完成时去掉箭头
		bar = strings.Repeat("=", pb.width)
	}

	// 计算 ETA
	elapsed := time.Since(pb.startTime)
	rate := float64(pb.current) / elapsed.Seconds()
	remaining := time.Duration(0)
	if rate > 0 {
		remaining = time.Duration(float64(pb.total-pb.current)/rate) * time.Second
	}
	etaStr := fmt.Sprintf("%02dm%02ds", int(remaining.Minutes()), int(remaining.Seconds())%60)

	// 颜色逻辑
	barColor := Cyan
	if percent >= 1.0 {
		barColor = Green
	}

	vulnColor := Green
	if pb.vulnCount > 0 {
		vulnColor = Red
	}

	fmt.Printf("%s%s %s [%s]%s %.0f%% | %d/%d | ETA: %s | Vulns: %s%d%s \n",
		Clear, // 清除行
		pb.description,
		barColor, bar, Reset,
		percent*100,
		pb.current, pb.total,
		etaStr,
		vulnColor, pb.vulnCount, Reset,
	)
}

func FormatVulnMsg(address string, vulns []string) string {
	return fmt.Sprintf(" %s🔴 Found %d Vulns in %s%s: %s",
		Red, len(vulns), Bold, address, Reset+strings.Join(vulns, ", "))
}
