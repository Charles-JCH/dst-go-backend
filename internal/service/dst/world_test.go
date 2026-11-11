package dst

import (
	"strings"
	"testing"
)

func TestNormalizeOverrides(t *testing.T) {
	got, err := normalizeOverrides(shardMaster, map[string]string{"klaus": "never", "walrus": "often"})
	if err != nil {
		t.Fatal(err)
	}
	// 默认存档中 walrus 为 often，与默认值相同的项不保存
	if len(got) != 1 || got["klaus"] != "never" {
		t.Errorf("normalizeOverrides = %v", got)
	}

	for _, bad := range []map[string]string{
		{"not_exist": "default"},
		{"klaus": `never", os.execute("rm -rf /") --`},
		{"wormattacks": "default"}, // 洞穴才有的设置
	} {
		if _, err := normalizeOverrides(shardMaster, bad); err == nil {
			t.Errorf("应拒绝 %v", bad)
		}
	}
	if _, err := normalizeOverrides("forest", nil); err == nil {
		t.Error("应拒绝未知分片")
	}
}

func TestRenderLevelData(t *testing.T) {
	text, err := renderLevelData(shardMaster, map[string]string{"klaus": "never"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`location="forest"`,
		"    klaus=\"never\",\n",
		"    has_ocean=true,\n", // 目录之外的键保留默认存档的字面量
		"    layout_mode=\"LinkNodesByKeys\",\n",
		"    year_of_the_varg=\"default\"\n  },", // 最后一项后面没有逗号
	} {
		if !strings.Contains(text, want) {
			t.Errorf("leveldataoverride.lua 缺少 %q", want)
		}
	}

	caves, err := renderLevelData(shardCaves, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(caves, `location="cave"`) || !strings.Contains(caves, `wormhole_prefab="tentacle_pillar"`) {
		t.Errorf("洞穴配置不正确:\n%s", caves)
	}
}
