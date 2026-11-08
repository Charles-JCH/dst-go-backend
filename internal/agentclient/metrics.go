package agentclient

import (
	"bufio"
	"game-panel/internal/protocol"
	"os"
	"strconv"
	"strings"
	"time"
)

// MetricsCollector 采集资源使用情况。CPU 使用率和网络速率按与上一次采样的差值计算，
// 所以需要保留上一次的计数；只在心跳协程里调用，不需要加锁
type MetricsCollector struct {
	diskPath string
	prevCPU  cpuTimes
	prevNet  netBytes
	prevAt   time.Time
}

type cpuTimes struct {
	total uint64
	idle  uint64
}

type netBytes struct {
	rx uint64
	tx uint64
}

// NewMetricsCollector 创建时先采样一次作为基准，第一次心跳就能算出使用率
func NewMetricsCollector(diskPath string) *MetricsCollector {
	return &MetricsCollector{
		diskPath: diskPath,
		prevCPU:  readCPUTimes(),
		prevNet:  readNetBytes(),
		prevAt:   time.Now(),
	}
}

// Collect 采集一次；某一项读取失败时为 0，不影响心跳
func (m *MetricsCollector) Collect() protocol.Metrics {
	now := time.Now()
	cpu := readCPUTimes()
	net := readNetBytes()
	ramTotal, ramAvailable := memInfo()
	_, diskUsed := diskUsage(m.diskPath)

	metrics := protocol.Metrics{
		CPUPercent: cpuPercent(m.prevCPU, cpu),
		RAMUsed:    max(ramTotal-ramAvailable, 0),
		DiskUsed:   diskUsed,
		Load1:      readLoad1(),
	}
	if seconds := now.Sub(m.prevAt).Seconds(); seconds > 0 {
		metrics.NetRx = int64(float64(counterDelta(m.prevNet.rx, net.rx)) / seconds)
		metrics.NetTx = int64(float64(counterDelta(m.prevNet.tx, net.tx)) / seconds)
	}

	m.prevCPU, m.prevNet, m.prevAt = cpu, net, now
	return metrics
}

// readCPUTimes 读取 /proc/stat 第一行的累计 CPU 时间
func readCPUTimes() cpuTimes {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}
	}
	line, _, _ := strings.Cut(string(data), "\n")
	// 格式：cpu  user nice system idle iowait irq softirq steal guest guest_nice
	// guest 已计入 user，只取前 8 项
	fields := strings.Fields(line)
	if len(fields) < 9 || fields[0] != "cpu" {
		return cpuTimes{}
	}
	var t cpuTimes
	for i, field := range fields[1:9] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}
		}
		t.total += v
		if i == 3 || i == 4 { // idle、iowait
			t.idle += v
		}
	}
	return t
}

// cpuPercent 两次采样之间的 CPU 使用率（0～100）
func cpuPercent(prev, cur cpuTimes) float64 {
	total := counterDelta(prev.total, cur.total)
	if total == 0 {
		return 0
	}
	busy := total - min(counterDelta(prev.idle, cur.idle), total)
	return float64(busy) * 100 / float64(total)
}

// readNetBytes 读取 /proc/net/dev，累加物理网卡的收发字节数；
// 回环和虚拟网卡（docker0、veth 等）会重复计算同一份流量，跳过
func readNetBytes() netBytes {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return netBytes{}
	}
	defer f.Close()

	var n netBytes
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// 格式：  eth0: rxBytes rxPackets ...（共 8 项接收）txBytes ...
		name, stats, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue // 表头
		}
		name = strings.TrimSpace(name)
		if _, err := os.Stat("/sys/devices/virtual/net/" + name); err == nil {
			continue
		}
		fields := strings.Fields(stats)
		if len(fields) < 9 {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr != nil || txErr != nil {
			continue
		}
		n.rx += rx
		n.tx += tx
	}
	return n
}

// readLoad1 读取 /proc/loadavg 的 1 分钟平均负载
func readLoad1() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return load
}

// counterDelta 累计计数的增量；计数被重置（如网卡重新加载）时返回 0
func counterDelta(prev, cur uint64) uint64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}
