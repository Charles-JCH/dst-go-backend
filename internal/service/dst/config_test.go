package dst

import (
	"game-panel/internal/model/entity"
	"game-panel/internal/protocol"
	"strings"
	"testing"
)

func TestRenderConfig(t *testing.T) {
	dst := &entity.Dst{Slot: 3, Name: "我的世界 & <冬天>", Description: "群：123", Password: "p@ss", MaxPlayers: 8, Mods: []string{"378160973"}}
	files, err := renderConfig(dst, "pds-g^KU_token", []string{"378160973", "1216718131", "378160973"})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(files))
	for _, f := range files {
		got[f.Name] = f.Content
	}

	ini := got[protocol.FileClusterINI]
	for _, want := range []string{
		"max_players = 8\n",
		"cluster_name = 我的世界 & <冬天>\n", // text/template 不做 HTML 转义
		"cluster_password = p@ss\n",
		"cluster_description = 群：123\n",
		"master_port = 10908\n", // 10888 + (3-1)*10
	} {
		if !strings.Contains(ini, want) {
			t.Errorf("cluster.ini 缺少 %q:\n%s", want, ini)
		}
	}
	if got[protocol.FileClusterToken] != "pds-g^KU_token" {
		t.Errorf("令牌内容不正确: %q", got[protocol.FileClusterToken])
	}
	wantMods := "return {\n  [\"workshop-378160973\"] = { enabled = true },\n}\n"
	if got[protocol.FileMasterModOverrides] != wantMods || got[protocol.FileCavesModOverrides] != wantMods {
		t.Errorf("modoverrides.lua 不正确:\n%s", got[protocol.FileMasterModOverrides])
	}
	setup := got[protocol.FileModsSetup]
	if strings.Count(setup, `ServerModSetup("378160973")`) != 1 || !strings.Contains(setup, `ServerModSetup("1216718131")`) {
		t.Errorf("模组下载列表应去重汇总:\n%s", setup)
	}
}
