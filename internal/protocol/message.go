package protocol

import "encoding/json"

const (
	TypePing    = "ping"
	TypeCommand = "command"
	TypeAck     = "ack"
	TypeLog     = "log"
	TypeResult  = "result"

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
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

type Result struct {
	Action   string `json:"action"`
	Slot     int    `json:"slot,omitempty"`
	Success  bool   `json:"success"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}
