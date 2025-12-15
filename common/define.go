package common

import (
	"encoding/json"
	"time"
)

// DataSourceStruct 数据源结构体定义
type DataSourceStruct struct {
	ID          string                `json:"id"`
	State       int                   `json:"state"`
	Name        string                `json:"name"`
	Description *string               `json:"description"` // 使用指针允许NULL
	Type        string                `json:"type"`
	Addr        *DataSourceAddrStruct `json:"addr"`
	Ctime       time.Time             `json:"ctime"`
	Utime       time.Time             `json:"utime"`
}

// DataSourceAddrStruct 数据源地址结构体定义
type DataSourceAddrStruct struct {
	UDP  *DataSourceUDPAddrStruct  `json:"udp,omitempty"`
	MQTT *DataSourceMQTTAddrStruct `json:"mqtt,omitempty"`
}

// DataSourceUDPAddrStruct UDP数据源地址结构体定义
type DataSourceUDPAddrStruct struct {
	Port        uint16 `json:"port"`
	MulticastIP string `json:"multicast_ip"`
}

// DataSourceMQTTAddrStruct MQTT数据源地址结构体定义
type DataSourceMQTTAddrStruct struct {
	Broker   string   `json:"broker"`
	ClientID string   `json:"client_id"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Topics   []string `json:"topics"`
}

// MarshalJSON 自定义序列化格式
func (m *DataSourceStruct) MarshalJSON() ([]byte, error) {
	type Alias DataSourceStruct // 避免递归调用
	aux := &struct {
		Ctime string `json:"ctime,omitempty"`
		Utime string `json:"utime,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	aux.Ctime = m.Ctime.Format("2006-01-02 15:04:05")
	aux.Utime = m.Utime.Format("2006-01-02 15:04:05")

	return json.Marshal(aux)
}

// DataSourceDetailStruct 数据源(详细)结构体定义
type DataSourceDetailStruct struct {
	DataSourceStruct
	IsRunning bool   `json:"is_running"`
	Message   string `json:"message"`
}

// MarshalJSON 自定义序列化格式
func (m *DataSourceDetailStruct) MarshalJSON() ([]byte, error) {
	type Alias DataSourceDetailStruct // 避免递归调用
	aux := &struct {
		Ctime string `json:"ctime,omitempty"`
		Utime string `json:"utime,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	aux.Ctime = m.Ctime.Format("2006-01-02 15:04:05")
	aux.Utime = m.Utime.Format("2006-01-02 15:04:05")

	return json.Marshal(aux)
}
