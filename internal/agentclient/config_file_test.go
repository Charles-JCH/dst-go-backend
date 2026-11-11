package agentclient

import (
	"game-panel/internal/protocol"
	"testing"
)

func TestConfigPath(t *testing.T) {
	e := NewExecutor(ExecutorConfig{ClusterRoot: "/home/charles/.klei/DoNotStarveTogether", ModsDir: "/home/charles/dst/mods"})

	cases := []struct {
		name    string
		want    string
		isValid bool
	}{
		{protocol.FileClusterINI, "/home/charles/.klei/DoNotStarveTogether/Cluster_2/cluster.ini", true},
		{protocol.FileMasterModOverrides, "/home/charles/.klei/DoNotStarveTogether/Cluster_2/Master/modoverrides.lua", true},
		{protocol.FileCavesLevelData, "/home/charles/.klei/DoNotStarveTogether/Cluster_2/Caves/leveldataoverride.lua", true},
		{protocol.FileModsSetup, "/home/charles/dst/mods/dedicated_server_mods_setup.lua", true},
		{"../../.ssh/authorized_keys", "", false},
		{"/etc/passwd", "", false},
		{"Master/server.ini", "", false},
	}
	for _, c := range cases {
		got, err := e.configPath(c.name, 2)
		if c.isValid != (err == nil) || got != c.want {
			t.Errorf("configPath(%q) = %q, %v; want %q, valid=%t", c.name, got, err, c.want, c.isValid)
		}
	}
}
