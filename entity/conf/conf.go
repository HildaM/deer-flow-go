package conf

import (
	"fmt"
	"log"
	"sync"

	"github.com/HildaM/logs/slog"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	// 全局 koanf 实例，使用 "." 作为键路径分隔符
	k = koanf.New(".")
	// 配置读写锁，确保并发安全
	configMu sync.RWMutex
	// 文件提供者
	f *file.File
	// 缓存的配置实例
	appConf *AppConfig
)

// Init 初始化配置
func Init() error {
	// 加载配置
	if err := loadConfig(); err != nil {
		return fmt.Errorf("Init config failed, load config err: %v", err)
	}

	// 启动配置文件监听
	startConfigWatch()

	// 初始化日志
	if err := slog.InitFile("logs/app.log", slog.WithLevel("debug"), slog.WithColor(false)); err != nil {
		return fmt.Errorf("Init log failed, err: %+v", err)
	}

	cfg := GetCfg()
	slog.Info("Init config: %+v", cfg)
	return nil
}

// loadConfig 加载配置
func loadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	// 创建文件提供者
	f = file.Provider("config.yaml")

	// 从根目录加载配置文件
	if err := k.Load(f, yaml.Parser()); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// 解析配置到结构体，使用 yaml 标签
	var config AppConfig
	if err := k.UnmarshalWithConf("", &config, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 更新全局配置实例
	appConf = &config
	return nil
}

// GetCfg 获取配置（无锁读取）
func GetCfg() *AppConfig {
	return appConf
}

// startConfigWatch 启动配置文件监听
func startConfigWatch() {
	if f == nil {
		log.Printf("file provider not initialized")
		return
	}

	// 监听文件变化并在变化时重新加载配置
	f.Watch(func(event interface{}, err error) {
		if err != nil {
			log.Printf("Config file watch error: %v", err)
			return
		}

		// 配置文件发生变化，重新加载
		log.Printf("Config file changed. Reloading...")

		// 创建新的 koanf 实例并重新加载配置
		configMu.Lock()

		k = koanf.New(".")
		if err := k.Load(f, yaml.Parser()); err != nil {
			log.Printf("Failed to load reloaded config: %v", err)
			configMu.Unlock()
			return
		}

		// 重新解析配置到结构体
		var config AppConfig
		if err := k.UnmarshalWithConf("", &config, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
			log.Printf("Failed to unmarshal reloaded config: %v", err)
			configMu.Unlock()
			return
		}

		// 更新全局配置实例
		appConf = &config

		configMu.Unlock()

		log.Printf("Config reloaded: %+v", config)
	})
}
