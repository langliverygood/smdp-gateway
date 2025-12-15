package datasource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"smdp-gateway/common"
	"smdp-gateway/utils"
	"time"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动
	"gorm.io/gorm"
)

var db *gorm.DB

type model struct {
	ID          string    `gorm:"column:id;type:varchar(32);primaryKey;not null"`
	State       int       `gorm:"column:state;type:integer;default:0"`
	Name        string    `gorm:"column:name;type:varchar(32);uniqueIndex;not null"`
	Description *string   `gorm:"column:description;type:varchar(256)"` // 使用指针允许NULL
	Type        string    `gorm:"column:type;type:varchar(16);not null"`
	Addr        string    `gorm:"column:addr;type:varchar(2048);not null"`
	Ctime       time.Time `gorm:"column:ctime;not null"`
	Utime       time.Time `gorm:"column:utime;not null"`
}

func (model) TableName() string {
	return "datasource"
}

func initSQLite(file string) error {
	dir := filepath.Dir(file)
	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("无法创建目录: %v", err)
		}
	}

	var err error
	db, err = gorm.Open(sqlite.Open(file), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(1)                  // 最大空闲连接数
	sqlDB.SetMaxOpenConns(1)                  // 最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Minute * 5) // 连接最大存活时间

	// 自动迁移表结构
	err = db.AutoMigrate(
		&model{},
	)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %w", err)
	}

	return nil
}

func closeSQLite() {
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
}

// var (
// 	errNameDuplicated = errors.New("规则名重复")
// 	errNameNotFound   = errors.New("该规则不存在")
// )

func insert(m *common.DataSourceStruct) error {
	m.ID = utils.GenerateUniqueIDV3(time.Now().GoString(), 8)
	m.Ctime = time.Now()
	m.Utime = m.Ctime

	table := &model{}
	table.ID = m.ID
	table.State = m.State
	table.Name = m.Name
	table.Description = m.Description
	table.Type = m.Type
	table.Ctime = m.Ctime
	table.Utime = m.Utime

	b, _ := json.Marshal(m.Addr)
	table.Addr = string(b)
	result := db.Create(table)

	return result.Error
}

func deleteByID(id string) error {
	result := db.Where("id = ?", id).Delete(&model{})

	return result.Error
}

func updateConfig(id string, c *common.DataSourceStruct) error {
	b, _ := json.Marshal(c.Addr)
	result := db.Model(&model{}).Where("id = ?", id).Updates(model{
		Name:        c.Name,
		Description: c.Description,
		Type:        c.Type,
		Addr:        string(b),
		Utime:       time.Now(),
	})

	return result.Error
}

func updateState(id string, state int) error {
	// 这里用map，防止s=0时没有更新
	result := db.Model(&model{}).Where("id = ?", id).Updates(map[string]interface{}{
		"State": state,
	})

	return result.Error
}

func selectByID(id string) ([]common.DataSourceStruct, error) {
	var ms []model

	if err := db.Model(ms).Where("id = ?", id).Find(&ms).Error; err != nil {
		return nil, err
	}

	results := make([]common.DataSourceStruct, 0)
	for _, m := range ms {
		result := common.DataSourceStruct{
			ID:          m.ID,
			State:       m.State,
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Type,
			Ctime:       m.Ctime,
			Utime:       m.Utime,
		}
		json.Unmarshal([]byte(m.Addr), &result.Addr)
		results = append(results, result)
	}

	return results, nil
}

func selectByName(state int, t string, name string, fuzzy bool, pageNumber, pageSize *int) ([]common.DataSourceStruct, error) {
	var ms []model

	if *pageNumber < 1 {
		*pageNumber = 1
	}
	if *pageSize < 1 {
		*pageSize = 10
	}
	if *pageSize > 500 {
		*pageSize = 500
	}

	records := db.Model(ms)
	if state == 0 || state == 1 {
		records = records.Where("state = ?", state)
	}
	if len(t) > 0 {
		records = records.Where("type = ?", t)
	}
	if len(name) > 0 {
		if fuzzy {
			records = records.Where("name LIKE ?", "%"+name+"%")
		} else {
			records = records.Where("name = ?", name)
		}
	}

	if err := records.Order("utime DESC").Offset((*pageNumber - 1) * *pageSize).Limit(*pageSize).Find(&ms).Error; err != nil {
		return nil, err
	}

	results := make([]common.DataSourceStruct, 0)
	for _, m := range ms {
		result := common.DataSourceStruct{
			ID:          m.ID,
			State:       m.State,
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Type,
			Ctime:       m.Ctime,
			Utime:       m.Utime,
		}
		json.Unmarshal([]byte(m.Addr), &result.Addr)
		results = append(results, result)
	}

	return results, nil
}

func selectCount(state int, t string, name string) (int64, error) {
	var results []model
	var total int64

	records := db.Model(results)

	if state == 0 || state == 1 {
		records = records.Where("state = ?", state)
	}

	if len(t) > 0 {
		records = records.Where("type = ?", t)
	}

	if len(name) > 0 {
		records = records.Where("name LIKE ?", "%"+name+"%")
	}

	if err := records.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func selectAll() ([]common.DataSourceStruct, error) {
	var ms []model

	if err := db.Model(ms).Order("utime DESC").Find(&ms).Error; err != nil {
		return nil, err
	}

	results := make([]common.DataSourceStruct, 0)
	for _, m := range ms {
		result := common.DataSourceStruct{
			ID:          m.ID,
			State:       m.State,
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Type,
			Ctime:       m.Ctime,
			Utime:       m.Utime,
		}
		json.Unmarshal([]byte(m.Addr), &result.Addr)
		results = append(results, result)
	}

	return results, nil
}

func selectOpenedOnly() ([]common.DataSourceStruct, error) {
	var ms []model

	if err := db.Model(ms).Where("state != ?", 0).Order("utime DESC").Find(&ms).Error; err != nil {
		return nil, err
	}

	results := make([]common.DataSourceStruct, 0)
	for _, m := range ms {
		result := common.DataSourceStruct{
			ID:          m.ID,
			State:       m.State,
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Type,
			Ctime:       m.Ctime,
			Utime:       m.Utime,
		}
		json.Unmarshal([]byte(m.Addr), &result.Addr)
		results = append(results, result)
	}

	return results, nil
}
