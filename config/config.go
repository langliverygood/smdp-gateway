package config

import (
	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

// Server 服务端配置
type Server struct {
	AdminAddr string `validate:"required"`
	DebugMode bool
}

// Logger 日志配置
type Logger struct {
	FileName             string `validate:"required"`
	MaxAgeByHour         int64  `validate:"required"` // 日志最大保留时间，单位：小时
	RotationTimeByMinute int64  `validate:"required"` // 日志分片周期，单位：分钟
	Level                string `validate:"oneof=debug info warn error"`
	JSONEncoder          bool
}

// MQTT MQTT协议配置
type MQTT struct {
	Broker   string `validate:"required"`
	ClientID string `validate:"required"`
	Password string `validate:"required"`
}

// Configuration 配置文件
type Configuration struct {
	Server Server `validate:"required"`
	Log    Logger `validate:"required"`
	MQTT   MQTT   `validate:"required"`
}

var global Configuration

// Parse 解析配置文件
func Parse(configPath string) error {
	if _, err := toml.DecodeFile(configPath, &global); err != nil {
		return err
	}

	if err := validator.New().Struct(global); err != nil {
		return err
	}

	return nil
}

// Global 返回配置文件的全局变量
func Global() *Configuration {
	return &global
}
