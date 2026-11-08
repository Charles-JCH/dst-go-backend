package agentclient

import (
	"slices"
	"strings"
	"testing"
)

func TestCleanLogLine(t *testing.T) {
	cases := map[string]string{
		"\x1b[1;32m[2026-11-08 10:00:00] [INFO]\x1b[0m 开始部署": "[2026-11-08 10:00:00] [INFO] 开始部署",
		"progress: 10%\rprogress: 20%\rprogress: 30%":        "progress: 30%",
		"windows line\r": "windows line",
		"  \x1b[0m  ":    "",
	}
	for in, want := range cases {
		if got := cleanLogLine(in); got != want {
			t.Errorf("cleanLogLine(%q) = %q, want %q", in, got, want)
		}
	}

	long := strings.Repeat("饥", maxLogLine) // 多字节字符，截断不能切坏
	got := cleanLogLine(long)
	if len(got) > maxLogLine+len(" …（已截断）") || !strings.HasSuffix(got, "（已截断）") {
		t.Errorf("超长行未截断，长度 %d", len(got))
	}
}

func TestLogFilterMergesProgress(t *testing.T) {
	var sent []string
	f := newLogFilter(func(line string) { sent = append(sent, line) })

	f.Write("[INFO] 下载 DST 服务端")
	f.Write("Update state (0x61) downloading, progress: 10.00")
	f.Write("Update state (0x61) downloading, progress: 20.00")  // 1 秒内，被合并
	f.Write("Update state (0x61) downloading, progress: 30.00")  // 1 秒内，替换上一条
	f.Write("[INFO] 下载完成")                                       // 先补发最后一条进度
	f.Write("Update state (0x61) downloading, progress: 99.00")  // 距补发不到 1 秒，被合并
	f.Write("Update state (0x61) downloading, progress: 100.00") // 结束时 Flush 补发
	f.Flush()

	want := []string{
		"[INFO] 下载 DST 服务端",
		"Update state (0x61) downloading, progress: 10.00",
		"Update state (0x61) downloading, progress: 30.00",
		"[INFO] 下载完成",
		"Update state (0x61) downloading, progress: 100.00",
	}
	if !slices.Equal(sent, want) {
		t.Errorf("sent = %q\nwant  %q", sent, want)
	}
}
