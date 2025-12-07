package datasource

import (
	"fmt"
	"os"
	"path/filepath"
	"smdp-gateway/common"
	"smdp-gateway/utils"
	"time"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动
	"gorm.io/gorm"
)

type model common.DataSourceStruct

func (model) TableName() string {
	return "datasource"
}

var db *gorm.DB

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

func insert(m *model) error {
	m.ID = utils.GenerateUniqueIDV3(time.Now().GoString(), 8)
	m.Ctime = time.Now()
	m.Utime = m.Ctime
	result := db.Create(m)

	return result.Error
}

func deleteByID(id string) error {
	result := db.Where("id = ?", id).Delete(&model{})

	return result.Error
}

func updateConfig(id string, m *model) error {
	result := db.Model(&model{}).Where("id = ?", id).Updates(model{
		Name:        m.Name,
		Description: m.Description,
		Type:        m.Type,
		Addr:        m.Addr,
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

func selectByID(id string) ([]model, error) {
	var results []model

	if err := db.Model(results).Where("id = ?", id).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func selectByName(state int, name string, fuzzy bool, pageNumber, pageSize *int) ([]model, error) {
	var results []model

	if *pageNumber < 1 {
		*pageNumber = 1
	}
	if *pageSize < 1 {
		*pageSize = 10
	}
	if *pageSize > 500 {
		*pageSize = 500
	}

	records := db.Model(results)
	if state == 0 || state == 1 {
		records = records.Where("state = ?", state)
	}
	if len(name) > 0 {
		if fuzzy {
			records = records.Where("name LIKE ?", "%"+name+"%")
		} else {
			records = records.Where("name = ?", name)
		}
	}

	if err := records.Order("utime DESC").Offset((*pageNumber - 1) * *pageSize).Limit(*pageSize).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func selectByConfig(t, addr string) ([]model, error) {
	var results []model

	if err := db.Model(results).Where("type = ? and addr = ?", t, addr).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func count(state int, name string) (int64, error) {
	var results []model
	var total int64

	records := db.Model(results)

	if state == 0 || state == 1 {
		records = records.Where("state = ?", state)
	}

	if len(name) > 0 {
		records = records.Where("name LIKE ?", "%"+name+"%")
	}

	if err := records.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func selectAll() ([]model, error) {
	var results []model

	if err := db.Model(results).Where("state != ?", 0).Order("utime DESC").Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func selectOpenedOnly() ([]model, error) {
	var results []model

	if err := db.Model(results).Order("utime DESC").Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
