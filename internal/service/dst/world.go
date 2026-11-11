package dst

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	"maps"
	"slices"
)

// 世界设置的分片：地面和洞穴各一份目录和模板
const (
	shardMaster = "master"
	shardCaves  = "caves"
)

// worldOptionsJSON 世界设置目录，由 scripts/gen_world_options.js 从游戏的 customize.lua 生成
//
//go:embed world_options.json
var worldOptionsJSON []byte

var worldCatalog = mustLoadWorldCatalog()

// worldShard 一个分片的世界设置目录
type worldShard struct {
	// Options 可修改的设置，顺序与游戏中一致；默认值取自默认存档
	Options []response.WorldOptionResp `json:"options"`
	// Fixed 不在目录中的键（如 has_ocean、由地面控制的季节），值为 Lua 字面量，原样写入
	Fixed map[string]string `json:"fixed"`
	// allowed 设置项 → 可选值，校验用户输入：值会写入 Lua 文件，只接受目录中的值
	allowed map[string]map[string]bool
}

// levelDataEntry 模板中 overrides 的一项，Value 为 Lua 字面量
type levelDataEntry struct {
	Key   string
	Value string
}

// normalizeOverrides 校验世界设置并去掉与默认值相同的项
func normalizeOverrides(shard string, overrides map[string]string) (map[string]string, error) {
	catalog := worldCatalog[shard]
	if catalog == nil {
		return nil, fmt.Errorf("未知的世界设置分片 %s", shard)
	}
	result := make(map[string]string, len(overrides))
	for name, value := range overrides {
		values, ok := catalog.allowed[name]
		if !ok {
			return nil, fmt.Errorf("未知的世界设置 %s", name)
		}
		if !values[value] {
			return nil, fmt.Errorf("世界设置 %s 的值无效", name)
		}
		if value != defaultOf(catalog, name) {
			result[name] = value
		}
	}
	return result, nil
}

// renderWorldFiles 生成地面和洞穴的 leveldataoverride.lua
func renderWorldFiles(master map[string]string, caves map[string]string) ([]protocol.ConfigFile, error) {
	masterFile, err := renderLevelData(shardMaster, master)
	if err != nil {
		return nil, err
	}
	cavesFile, err := renderLevelData(shardCaves, caves)
	if err != nil {
		return nil, err
	}
	return []protocol.ConfigFile{
		{Name: protocol.FileMasterLevelData, Content: masterFile},
		{Name: protocol.FileCavesLevelData, Content: cavesFile},
	}, nil
}

// renderLevelData 生成 leveldataoverride.lua：目录中的设置取用户值或默认值，其余键保留默认存档的值
func renderLevelData(shard string, overrides map[string]string) (string, error) {
	catalog := worldCatalog[shard]
	values := maps.Clone(catalog.Fixed)
	for _, o := range catalog.Options {
		value := o.Default
		if v, ok := overrides[o.Name]; ok {
			value = v
		}
		values[o.Name] = `"` + value + `"` // 值已按目录校验，不含引号和反斜杠
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	entries := make([]levelDataEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, levelDataEntry{Key: key, Value: values[key]})
	}
	return render("leveldataoverride_"+shard+".lua.tmpl", entries)
}

func mustLoadWorldCatalog() map[string]*worldShard {
	var catalog map[string]*worldShard
	if err := json.Unmarshal(worldOptionsJSON, &catalog); err != nil {
		panic(fmt.Sprintf("世界设置目录解析失败: %v", err))
	}
	for _, shard := range []string{shardMaster, shardCaves} {
		s := catalog[shard]
		if s == nil {
			panic("世界设置目录缺少分片 " + shard)
		}
		s.allowed = make(map[string]map[string]bool, len(s.Options))
		for _, o := range s.Options {
			values := make(map[string]bool, len(o.Options))
			for _, v := range o.Options {
				values[v.Value] = true
			}
			s.allowed[o.Name] = values
		}
	}
	return catalog
}

func defaultOf(catalog *worldShard, name string) string {
	for _, o := range catalog.Options {
		if o.Name == name {
			return o.Default
		}
	}
	return ""
}
