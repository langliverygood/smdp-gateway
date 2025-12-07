package datasource

var validDataSourceType map[string]uint8 = make(map[string]uint8)

// Init 初始化资源
func Init() {
	initSQLite("./database/datasource.db")
	initMQTT()

	validDataSourceType["UDP"] = 1
	validDataSourceType["MQTT"] = 1
}

// Release 释放资源
func Release() {
	closeMQTT()
	closeSQLite()
}
