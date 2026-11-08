package agentclient

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxLogLine       = 4 * 1024    // 单行日志上限，超出截断
	progressInterval = time.Second // 进度行在这段时间内只发送最后一条
)

var (
	// ansiPattern 终端颜色等控制序列，如 \033[1;32m
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	// progressPattern steamcmd 等工具的下载进度行
	progressPattern = regexp.MustCompile(`(?i)progress:`)
)

// logFilter 整理脚本输出再发送：去掉颜色代码、截断超长行，并合并刷屏的进度行。
// 只在执行命令的协程里使用，不需要加锁
type logFilter struct {
	send         func(line string)
	pending      string // 被合并、还没发送的最新进度行
	hasPending   bool
	lastProgress time.Time
}

func newLogFilter(send func(line string)) *logFilter {
	return &logFilter{send: send}
}

func (f *logFilter) Write(line string) {
	line = cleanLogLine(line)
	if line == "" {
		return
	}

	if !progressPattern.MatchString(line) {
		f.Flush() // 先补发被合并的进度行，保证先后顺序
		f.send(line)
		return
	}

	now := time.Now()
	if now.Sub(f.lastProgress) < progressInterval {
		f.pending, f.hasPending = line, true
		return
	}
	f.pending, f.hasPending = "", false
	f.lastProgress = now
	f.send(line)
}

// Flush 发送最后一条被合并的进度行，命令结束时调用
func (f *logFilter) Flush() {
	if !f.hasPending {
		return
	}
	line := f.pending
	f.pending, f.hasPending = "", false
	f.lastProgress = time.Now()
	f.send(line)
}

// cleanLogLine 去掉颜色代码；带 \r 的行是在原地刷新的进度，只保留最后一段
func cleanLogLine(line string) string {
	if i := strings.LastIndex(strings.TrimRight(line, "\r"), "\r"); i >= 0 {
		line = line[i+1:]
	}
	line = strings.TrimRight(ansiPattern.ReplaceAllString(line, ""), "\r \t")
	if len(line) > maxLogLine {
		line = strings.ToValidUTF8(line[:maxLogLine], "") + " …（已截断）"
	}
	if !utf8.ValidString(line) {
		line = strings.ToValidUTF8(line, "")
	}
	return line
}
