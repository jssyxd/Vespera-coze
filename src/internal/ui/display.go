package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
	Bold   = "\033[1m"
)

var (
	lastLineLength int
	mu             sync.Mutex
)

func PrintBanner() {
	banner := `
__      __                             
\ \    / /                             
 \ \  / /__  ___ _ __   ___ _ __ __ _  
  \ \/ / _ \/ __| '_ \ / _ \ '__/ _` + "`" + ` | 
   \  /  __/\__ \ |_) |  __/ | | (_| | 
    \/ \___||___/ .__/ \___|_|  \__,_| 
                | |                    
                |_|                    
`
	fmt.Println(Cyan + banner + Reset)
	fmt.Println(Gray + "  v1.0.0 - Modular EVM Smart Contract Vulnerability Detection Framework" + Reset)
	fmt.Println()
}

func clearLine() {
	fmt.Print("\r\033[K")
}

func UpdateStatus(format string, a ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	msg := fmt.Sprintf(format, a...)
	clearLine()

	// Truncate if too long to avoid wrapping issues (simple approach)
	if len(msg) > 100 {
		msg = msg[:97] + "..."
	}

	fmt.Print(Cyan + "⚡ " + msg + Reset)
	lastLineLength = len(msg)
}

func LogSuccess(format string, a ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	clearLine()
	fmt.Printf(Green+"[SUCCESS] "+Reset+format+"\n", a...)
}

func LogVuln(address string, count int, aiConfirmed int) {
	mu.Lock()
	defer mu.Unlock()
	clearLine()
	fmt.Printf(Red+"[VULN FOUND] "+Reset+"%s | Detectors: %d | AI Confirmed: %d\n", address, count, aiConfirmed)
}

func LogInfo(format string, a ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	clearLine()
	fmt.Printf(Blue+"[INFO] "+Reset+format+"\n", a...)
}

func LogError(format string, a ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	clearLine()
	fmt.Printf(Red+"[ERROR] "+Reset+format+"\n", a...)
}

func StartSpinner(msg string) chan bool {
	stop := make(chan bool)
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				mu.Lock()
				clearLine()
				fmt.Printf(Cyan+"%s %s"+Reset, frames[i%len(frames)], msg)
				mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				i++
			}
		}
	}()
	return stop
}

func PrintStats(total, success, failed, vulns int, duration time.Duration) {
	fmt.Println()
	fmt.Println(Gray + strings.Repeat("─", 50) + Reset)
	fmt.Printf("🏁 Scan Completed in %s\n", duration)
	fmt.Printf("📊 Total: %d | ✅ Success: %d | ❌ Failed: %d | 🛡️  Vulns Found: %d\n", total, success, failed, vulns)
	fmt.Println(Gray + strings.Repeat("─", 50) + Reset)
}
