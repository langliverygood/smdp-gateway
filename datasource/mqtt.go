package datasource

import (
	"smdp-gateway/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClient mqtt.Client

// GetMQTTClient 获取mqtt客户端对象
func GetMQTTClient() mqtt.Client {
	return mqttClient
}

func initMQTT() {
	// 创建客户端选项
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Global().MQTT.Broker)
	opts.SetClientID(config.Global().MQTT.ClientID)
	opts.SetUsername(config.Global().MQTT.ClientID)
	opts.SetPassword(config.Global().MQTT.Password)

	// 创建客户端
	mqttClient = mqtt.NewClient(opts)

	// 连接
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

func closeMQTT() {
	mqttClient.Disconnect(200)
}
