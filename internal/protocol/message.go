package protocol

import "encoding/json"

const (
	TypePing    = "ping"
	TypeCommand = "command"
	TypeAck     = "ack"
	TypeLog     = "log"
	TypeResult  = "result"
	// TypeUninstall 平台删除服务器时通知 Agent 卸载自己，不回传结果
	TypeUninstall = "uninstall"

	ActionDeploy = "deploy"
	ActionInit   = "init"
	ActionStart  = "start"
	ActionStop   = "stop"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionStatus = "status"
)

type Envelope struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type CmdPayload struct {
	Action       string `json:"action"`
	Slot         int    `json:"slot,omitempty"`
	ClusterToken string `json:"clusterToken,omitempty"`
}

type Log struct {
	Content string `json:"content"`
}

type Ack struct {
	IsAccepted bool   `json:"accepted"`
	Reason     string `json:"reason,omitempty"`
}

type Result struct {
	Action    string `json:"action"`
	Slot      int    `json:"slot,omitempty"`
	IsSuccess bool   `json:"success"`
	ExitCode  int    `json:"exitCode"`
	Error     string `json:"error,omitempty"`
}
