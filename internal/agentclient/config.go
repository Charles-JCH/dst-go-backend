package agentclient

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Executor ExecutorConfig `mapstructure:"executor"`
	Identity IdentityConfig `mapstructure:"identity"`
	Service  ServiceConfig  `mapstructure:"service"`
	SysInfo  SysInfoConfig  `mapstructure:"sysinfo"`
}

type ServerConfig struct {
	WSURL             string        `mapstructure:"ws_url"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	ConnectTimeout    time.Duration `mapstructure:"connect_timeout"`
	ReconnectInterval time.Duration `mapstructure:"reconnect_interval"`
}

type ExecutorConfig struct {
	ScriptPath  string        `mapstructure:"script_path"`
	ExecTimeout time.Duration `mapstructure:"exec_timeout"`
}

type IdentityConfig struct {
	Path string `mapstructure:"path"`
}

type ServiceConfig struct {
	Name       string `mapstructure:"name"`
	UnitPath   string `mapstructure:"unit_path"`
	InstallDir string `mapstructure:"install_dir"`
}

type SysInfoConfig struct {
	// DiskPath 游戏文件所在目录，上报它所在分区的容量；目录还不存在时取最近一级存在的上级目录
	DiskPath string `mapstructure:"disk_path"`
}

// Load 读取并解析配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 支持环境变量覆盖
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 旧版本安装的配置没有 sysinfo，按根分区统计
	v.SetDefault("sysinfo.disk_path", "/")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("配置文件读取失败: %w", err)
	}

	// 反序列化
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %w", err)
	}

	return &cfg, nil
}
