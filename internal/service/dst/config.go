package dst

import (
	"bytes"
	"embed"
	"game-panel/internal/model/entity"
	"game-panel/internal/protocol"
	"slices"
	"text/template"
)

// 端口规则与 dst.sh 一致：槽位 N 的分片端口为 DST_SHARD_PORT + (N-1) * DST_PORT_STEP
const (
	shardPortBase = 10888
	slotPortStep  = 10
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templates 存档配置文件模板，占位符由存档信息替换；名称、描述、密码已校验不含换行，可直接写入
var templates = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// clusterINIData cluster.ini 模板的数据
type clusterINIData struct {
	Name        string
	Description string
	Password    string
	MaxPlayers  int
	MasterPort  int
}

// renderConfig 生成存档的全部配置文件；serverMods 为该服务器所有存档的模组（含本存档），
// 模组下载列表是所有存档共用的，必须汇总后整份生成
func renderConfig(dst *entity.Dst, clusterToken string, serverMods []string) ([]protocol.ConfigFile, error) {
	clusterINI, err := render("cluster.ini.tmpl", clusterINIData{
		Name:        dst.Name,
		Description: dst.Description,
		Password:    dst.Password,
		MaxPlayers:  dst.MaxPlayers,
		MasterPort:  shardPortBase + (dst.Slot-1)*slotPortStep,
	})
	if err != nil {
		return nil, err
	}
	modOverrides, err := render("modoverrides.lua.tmpl", dst.Mods)
	if err != nil {
		return nil, err
	}
	modsSetup, err := render("dedicated_server_mods_setup.lua.tmpl", uniqueSorted(serverMods))
	if err != nil {
		return nil, err
	}

	return []protocol.ConfigFile{
		{Name: protocol.FileClusterINI, Content: clusterINI},
		{Name: protocol.FileClusterToken, Content: clusterToken},
		{Name: protocol.FileMasterModOverrides, Content: modOverrides},
		{Name: protocol.FileCavesModOverrides, Content: modOverrides},
		{Name: protocol.FileModsSetup, Content: modsSetup},
	}, nil
}

func render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func uniqueSorted(items []string) []string {
	result := slices.Clone(items)
	slices.Sort(result)
	return slices.Compact(result)
}
