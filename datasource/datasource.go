package datasource

import (
	"fmt"
	"net"
	"net/url"
	"smdp-gateway/common"
	"strings"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var validDataSourceType map[string]uint8 = make(map[string]uint8)

type sourceRecord interface {
	Add(id string, addr *common.DataSourceAddrStruct) error
	Delete()
	DeleteAll()
}

// UDP数据源记录
type udpSourceRecordStruct struct {
	conns                map[string]*net.UDPConn                    // key 是 id
	configs              map[string]*common.DataSourceUDPAddrStruct // key 是 id
	listenedPort         map[uint16]uint8                           // key 是端口
	joinedMulticastGroup map[string]uint8                           // key 是组播地址
	multicastConfigs     map[string]*multicastConfig                // key 是组播地址
}

func (r *udpSourceRecordStruct) Add(id string, addr *common.DataSourceAddrStruct) error {
	var err error
	_, portExist := r.listenedPort[addr.UDP.Port]
	if !portExist { // 这个端口还没有监听
		localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", addr.UDP.Port))
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", localAddr)
		if err != nil {
			return err
		}
		r.conns[id] = conn
		r.listenedPort[addr.UDP.Port] = 1
	} else {
		err = fmt.Errorf("UDP连接已存在, port:%d, multicastIP:%s", addr.UDP.Port, addr.UDP.MulticastIP)
	}

	if len(addr.UDP.MulticastIP) > 0 {
		_, groupExist := r.joinedMulticastGroup[addr.UDP.MulticastIP]
		if !groupExist {
			err = nil // 走到这里，说明连接没有重复
			var cf *multicastConfig
			cf, err = joinMulticastGroup(addr.UDP.MulticastIP, r.conns[id])
			if err != nil {
				return nil
			}
			r.joinedMulticastGroup[addr.UDP.MulticastIP] = 1
			r.multicastConfigs[id] = cf
		}
	}
	return err
}

func (r *udpSourceRecordStruct) Delete(id string) {
	if v, ok := r.conns[id]; ok {
		v.Close()
		delete(r.conns, id)
		if config, ok := r.configs[id]; ok {
			delete(r.listenedPort, config.Port)
			if _, ok := r.joinedMulticastGroup[config.MulticastIP]; ok {
				// 离开组播
				syscall.SetsockoptIPMreq(r.multicastConfigs[config.MulticastIP].Fd,
					syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, r.multicastConfigs[config.MulticastIP].IPMreq)
				delete(r.joinedMulticastGroup, config.MulticastIP)
				delete(r.multicastConfigs, config.MulticastIP)
			}
			delete(r.configs, id)
		}
	}
}

func (r *udpSourceRecordStruct) DeleteAll() {
	// 离开组播
	for _, v := range r.multicastConfigs {
		syscall.SetsockoptIPMreq(v.Fd, syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, v.IPMreq)
	}
	// 断开连接
	for _, v := range r.conns {
		v.Close()
	}
	r.conns = make(map[string]*net.UDPConn)
	r.configs = make(map[string]*common.DataSourceUDPAddrStruct)
	r.listenedPort = make(map[uint16]uint8)
	r.joinedMulticastGroup = make(map[string]uint8)
	r.multicastConfigs = make(map[string]*multicastConfig)
}

type mqttSourceRecordStruct struct {
	conns   map[string]mqtt.Client
	configs map[string]*common.DataSourceMQTTAddrStruct
	client  map[string]map[string]uint8 // key: broker value: clientID
}

func (r *mqttSourceRecordStruct) Add(id string, addr *common.DataSourceAddrStruct) error {
	broker := addr.MQTT.Broker
	// 解析为 URL
	u, err := url.Parse(broker)
	if err == nil {
		broker = u.Host
	} else {
		if strings.Contains(broker, ":") {
			if _, _, err := net.SplitHostPort(broker); err != nil {
				return err
			}
		} else {
			if _, err := net.ResolveIPAddr("ip", broker); err != nil {
				return err
			}
		}
	}

	if _, ok := r.client[broker]; !ok {
		r.client[broker] = make(map[string]uint8)
	}

	if _, ok := r.client[broker][addr.MQTT.ClientID]; !ok {
		opts := mqtt.NewClientOptions()
		opts.AddBroker(broker)
		opts.SetClientID(addr.MQTT.ClientID)
		// TODD 后面要支持 两种认证方式
		opts.SetUsername(addr.MQTT.ClientID)
		opts.SetPassword(addr.MQTT.Password)
		// 创建客户端
		mqttClient := mqtt.NewClient(opts)
		// 连接
		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			return token.Error()
		}

		for _, t := range addr.MQTT.Topics {
			mqttClient.Subscribe(t, 0, nil)
		}

		r.conns[id] = mqttClient
		r.configs[id] = addr.MQTT
		r.client[broker][addr.MQTT.ClientID] = 1
	} else {
		return fmt.Errorf("MQTT连接已存在, broker:%s, clientID:%s", addr.MQTT.Broker, addr.MQTT.ClientID)
	}
	return nil
}

func (r *mqttSourceRecordStruct) Delete(id string) {
	if conn, ok := r.conns[id]; ok {
		conn.Disconnect(0)
		delete(r.conns, id)
		if config, ok := r.configs[id]; ok {
			if broker, ok := r.client[config.Broker]; ok {
				if _, ok := broker[config.ClientID]; ok {
					delete(r.client[config.Broker], config.ClientID)
				}
				delete(r.client, config.Broker)
			}
			delete(r.configs, id)
		}
	}
}

func (r *mqttSourceRecordStruct) DeleteAll() {
	for _, v := range r.conns {
		v.Disconnect(0)
	}
	r.conns = make(map[string]mqtt.Client)
	r.configs = make(map[string]*common.DataSourceMQTTAddrStruct)
	r.client = make(map[string]map[string]uint8)
}

// 数据源记录
var dataSourceDetail map[string]*common.DataSourceDetailStruct
var udpSourceRecord *udpSourceRecordStruct
var mqttSourceRecord *mqttSourceRecordStruct

// Init 初始化资源
func Init() {
	initSQLite("./database/datasource.db")

	validDataSourceType["UDP"] = 1
	validDataSourceType["MQTT"] = 1

	dataSourceDetail = make(map[string]*common.DataSourceDetailStruct)

	udpSourceRecord = &udpSourceRecordStruct{
		conns:                make(map[string]*net.UDPConn),
		configs:              make(map[string]*common.DataSourceUDPAddrStruct),
		listenedPort:         make(map[uint16]uint8),
		joinedMulticastGroup: make(map[string]uint8),
		multicastConfigs:     make(map[string]*multicastConfig),
	}
	mqttSourceRecord = &mqttSourceRecordStruct{
		conns:   make(map[string]mqtt.Client),
		configs: make(map[string]*common.DataSourceMQTTAddrStruct),
		client:  make(map[string]map[string]uint8),
	}
	//
	ds, err := selectOpenedOnly()
	if err != nil {
		panic(err)
	}
	for _, d := range ds {
		dataSourceDetail[d.ID] = &common.DataSourceDetailStruct{}
		dataSourceDetail[d.ID].ID = d.ID
		dataSourceDetail[d.ID].State = d.State
		dataSourceDetail[d.ID].Name = d.Name
		dataSourceDetail[d.ID].Description = d.Description
		dataSourceDetail[d.ID].Type = d.Type
		dataSourceDetail[d.ID].Addr = d.Addr
		dataSourceDetail[d.ID].Ctime = d.Ctime
		dataSourceDetail[d.ID].Utime = d.Utime
		dataSourceDetail[d.ID].IsRunning = true
		dataSourceDetail[d.ID].Message = "OK"
		switch d.Type {
		case "UDP":
			if err := udpSourceRecord.Add(d.ID, d.Addr); err != nil {
				dataSourceDetail[d.ID].IsRunning = false
				dataSourceDetail[d.ID].Message = err.Error()
			}
		case "MQTT":
			if err := mqttSourceRecord.Add(d.ID, d.Addr); err != nil {
				dataSourceDetail[d.ID].IsRunning = false
				dataSourceDetail[d.ID].Message = err.Error()
			}
		}
	}
}

// Release 释放资源
func Release() {
	closeSQLite()
	dataSourceDetail = make(map[string]*common.DataSourceDetailStruct)
	udpSourceRecord.DeleteAll()
	mqttSourceRecord.DeleteAll()
}

// getDataSourceDetail 根据ID获取数据源详情
func getDataSourceDetail(id string) *common.DataSourceDetailStruct {
	if detail, ok := dataSourceDetail[id]; ok {
		return detail
	}
	return nil
}
