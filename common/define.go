package common

import (
	"encoding/json"
	"time"
)

// DataSourceStruct 数据源结构体定义
type DataSourceStruct struct {
	ID          string    `gorm:"column:id;type:varchar(32);primaryKey;not null" json:"id"`
	State       int       `gorm:"column:state;type:integer;default:0" json:"state"`
	Name        string    `gorm:"column:name;type:varchar(32);uniqueIndex;not null" json:"name"`
	Description *string   `gorm:"column:description;type:varchar(256)" json:"description"` // 使用指针允许NULL
	Type        string    `gorm:"column:type;type:varchar(16);not null" json:"type"`
	Addr        string    `gorm:"column:addr;type:varchar(2048);not null" json:"addr"`
	Ctime       time.Time `gorm:"column:ctime;not null" json:"ctime"`
	Utime       time.Time `gorm:"column:utime;not null" json:"utime"`
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
