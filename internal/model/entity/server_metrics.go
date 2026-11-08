package entity

import "time"

// ServerMetrics 服务器某一时刻的资源使用情况，由 Agent 随心跳上报，保存在 Redis 中（不建表）。
// ramUsed、diskUsed 单位为字节，netRx、netTx 单位为字节/秒
type ServerMetrics struct {
	Time       time.Time `json:"t"`
	CPUPercent float64   `json:"cpu"`
	RAMUsed    int64     `json:"ram"`
	DiskUsed   int64     `json:"disk"`
	Load1      float64   `json:"load"`
	NetRx      int64     `json:"rx"`
	NetTx      int64     `json:"tx"`
}
