package datasource

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"smdp-gateway/common"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var validDataSourceType map[string]uint8 = make(map[string]uint8)

// DataMessage 数据消息结构
type DataMessage struct {
	SourceID string // 数据源ID
	Data     []byte // 原始数据
	From     string // 数据来源地址（UDP为客户端地址，MQTT为topic）
}

// DataHandler 数据处理函数类型
type DataHandler func(msg *DataMessage)

// HandlerConfig 处理者配置
type HandlerConfig struct {
	Handler    DataHandler // 处理函数
	Sequential bool        // 是否顺序处理（true=顺序，false=并发）
	BufferSize int         // 顺序处理时的缓冲区大小，0表示使用默认值
}

// handlerInfo 内部处理者信息
type handlerInfo struct {
	config   HandlerConfig
	dataChan chan *DataMessage // 顺序处理时使用的通道
	ctx      context.Context   // 控制协程生命周期
	cancel   context.CancelFunc
	wg       sync.WaitGroup // 等待处理协程结束
}

// DataBroadcaster 数据广播器，支持多个处理者订阅
type DataBroadcaster struct {
	handlers map[string]*handlerInfo // key是处理者ID
	mu       sync.RWMutex
}

// NewDataBroadcaster 创建新的数据广播器
func NewDataBroadcaster() *DataBroadcaster {
	return &DataBroadcaster{
		handlers: make(map[string]*handlerInfo),
	}
}

// Subscribe 订阅数据（并发处理），返回取消订阅的函数
func (db *DataBroadcaster) Subscribe(handlerID string, handler DataHandler) func() {
	return db.SubscribeWithConfig(handlerID, HandlerConfig{
		Handler:    handler,
		Sequential: false, // 并发处理
	})
}

// SubscribeSequential 订阅数据（顺序处理），返回取消订阅的函数
func (db *DataBroadcaster) SubscribeSequential(handlerID string, handler DataHandler, bufferSize int) func() {
	if bufferSize <= 0 {
		bufferSize = 100 // 默认缓冲区大小
	}
	return db.SubscribeWithConfig(handlerID, HandlerConfig{
		Handler:    handler,
		Sequential: true,
		BufferSize: bufferSize,
	})
}

// SubscribeWithConfig 使用配置订阅数据，返回取消订阅的函数
func (db *DataBroadcaster) SubscribeWithConfig(handlerID string, config HandlerConfig) func() {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 如果已存在，先清理
	if existing, exists := db.handlers[handlerID]; exists {
		existing.cancel()
		existing.wg.Wait()
	}

	ctx, cancel := context.WithCancel(context.Background())
	info := &handlerInfo{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	if config.Sequential {
		// 顺序处理：创建通道和处理协程
		bufferSize := config.BufferSize
		if bufferSize <= 0 {
			bufferSize = 100
		}
		info.dataChan = make(chan *DataMessage, bufferSize)

		info.wg.Add(1)
		go func() {
			defer info.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-info.dataChan:
					func() {
						defer func() {
							if r := recover(); r != nil {
								// 处理panic，避免影响其他处理者
							}
						}()
						config.Handler(msg)
					}()
				}
			}
		}()
	}

	db.handlers[handlerID] = info

	// 返回取消订阅的函数
	return func() {
		db.mu.Lock()
		if info, exists := db.handlers[handlerID]; exists {
			info.cancel()
			delete(db.handlers, handlerID)
			db.mu.Unlock()
			info.wg.Wait() // 等待处理协程结束
		} else {
			db.mu.Unlock()
		}
	}
}

// Broadcast 广播数据给所有订阅者
func (db *DataBroadcaster) Broadcast(msg *DataMessage) {
	db.mu.RLock()
	handlers := make([]*handlerInfo, 0, len(db.handlers))
	for _, info := range db.handlers {
		handlers = append(handlers, info)
	}
	db.mu.RUnlock()

	// 如果没有处理者，直接丢弃数据
	if len(handlers) == 0 {
		return
	}

	// 分别处理并发和顺序处理者
	for _, info := range handlers {
		if info.config.Sequential {
			// 顺序处理：发送到通道（非阻塞）
			select {
			case info.dataChan <- msg:
			default:
				// 通道满了，丢弃数据（可以根据需要调整策略）
			}
		} else {
			// 并发处理：启动新协程
			go func(handler DataHandler) {
				defer func() {
					if r := recover(); r != nil {
						// 处理panic，避免影响其他处理者
					}
				}()
				handler(msg)
			}(info.config.Handler)
		}
	}
}

// GetHandlerCount 获取当前处理者数量
func (db *DataBroadcaster) GetHandlerCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.handlers)
}

// DataProcessor 数据处理接口（预留，兼容旧接口）
type DataProcessor interface {
	ProcessData(msg *DataMessage) error
}

type sourceRecord interface {
	Add(id string, addr *common.DataSourceAddrStruct) error
	Delete(id string)
	DeleteAll()
}

// UDP数据源记录
type udpSourceRecordStruct struct {
	conns                map[uint16]*net.UDPConn                    // key 是端口
	configs              map[string]*common.DataSourceUDPAddrStruct // key 是 id
	listenedPort         map[uint16]uint8                           // key 是端口
	joinedMulticastGroup map[string]uint8                           // key 是组播地址
	multicastConfigs     map[string]*multicastConfig                // key 是组播地址

	// 协程管理
	contexts    map[string]context.Context    // key 是 id，用于控制协程退出
	cancels     map[string]context.CancelFunc // key 是 id，用于取消协程
	wg          sync.WaitGroup                // 等待所有协程结束
	broadcaster *DataBroadcaster              // 数据广播器，替代通道
	mu          sync.RWMutex                  // 保护并发访问
}

func (r *udpSourceRecordStruct) Add(id string, addr *common.DataSourceAddrStruct) error {
	// 单播和组播其实是支持端口复用的。第一版先简单做，认为会端口冲突，后面修改逻辑。
	// 下面是支持 端口复用的代码，先注释
	// var err error
	// _, portExist := r.listenedPort[addr.UDP.Port]
	// if !portExist { // 这个端口还没有监听
	// 	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", addr.UDP.Port))
	// 	if err != nil {
	// 		return err
	// 	}
	// 	conn, err := net.ListenUDP("udp", localAddr)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.conns[addr.UDP.Port] = conn
	// 	r.listenedPort[addr.UDP.Port] = 1
	// } else {
	// 	err = fmt.Errorf("UDP连接已存在, port:%d, multicastIP:%s", addr.UDP.Port, addr.UDP.MulticastIP)
	// }

	// if len(addr.UDP.MulticastIP) > 0 {
	// 	_, groupExist := r.joinedMulticastGroup[addr.UDP.MulticastIP]
	// 	if !groupExist {
	// 		err = nil // 走到这里，说明连接没有重复
	// 		var cf *multicastConfig
	// 		cf, err = joinMulticastGroup(addr.UDP.MulticastIP, r.conns[addr.UDP.Port])
	// 		if err != nil {
	// 			fmt.Println(err)
	// 			return err
	// 		}
	// 		r.joinedMulticastGroup[addr.UDP.MulticastIP] = 1
	// 		r.multicastConfigs[addr.UDP.MulticastIP] = cf
	// 	}
	// }

	// if err == nil {
	// 	r.configs[id] = addr.UDP
	// }
	//下面是不支持 端口复用的代码，先注释
	if _, portExist := r.listenedPort[addr.UDP.Port]; portExist {
		return fmt.Errorf("UDP端口已被占用, port:%d", addr.UDP.Port)
	}

	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", addr.UDP.Port))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return err
	}
	r.conns[addr.UDP.Port] = conn
	r.listenedPort[addr.UDP.Port] = 1

	if len(addr.UDP.MulticastIP) > 0 {
		_, groupExist := r.joinedMulticastGroup[addr.UDP.MulticastIP]
		if !groupExist {
			cf, err := joinMulticastGroup(addr.UDP.MulticastIP, r.conns[addr.UDP.Port])
			if err != nil {
				return err
			}
			r.joinedMulticastGroup[addr.UDP.MulticastIP] = 1
			r.multicastConfigs[addr.UDP.MulticastIP] = cf
		}
	}
	r.configs[id] = addr.UDP

	// 启动数据接收协程
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.contexts[id] = ctx
	r.cancels[id] = cancel
	r.mu.Unlock()

	r.wg.Add(1)
	go r.receiveUDPData(ctx, id, addr.UDP.Port)

	return err
}

// receiveUDPData UDP数据接收协程
func (r *udpSourceRecordStruct) receiveUDPData(ctx context.Context, id string, port uint16) {
	defer r.wg.Done()

	r.mu.RLock()
	conn, exists := r.conns[port]
	r.mu.RUnlock()

	if !exists {
		return
	}

	buffer := make([]byte, 4096) // 4KB缓冲区

	for {
		select {
		case <-ctx.Done():
			// 协程被取消，退出
			return
		default:
			// 设置读取超时，避免阻塞
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				// 检查是否是超时错误
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // 超时继续循环
				}
				// 其他错误，可能连接已关闭
				return
			}

			// 创建数据消息
			msg := &DataMessage{
				SourceID: id,
				Data:     make([]byte, n),
				From:     addr.String(),
			}
			copy(msg.Data, buffer[:n])

			fmt.Printf("[%s]Receive From UDP (%s), Size:%d\n", conn.LocalAddr(), addr.String(), n)

			// 广播数据给所有订阅者（如果没有订阅者会自动丢弃）
			r.broadcaster.Broadcast(msg)
		}
	}
}

func (r *udpSourceRecordStruct) Delete(id string) {
	if config, ok := r.configs[id]; ok {
		// 停止协程
		r.mu.Lock()
		if cancel, exists := r.cancels[id]; exists {
			cancel()
			delete(r.contexts, id)
			delete(r.cancels, id)
		}
		r.mu.Unlock()

		port := config.Port
		if v, ok := r.conns[port]; ok {
			v.Close()
			delete(r.conns, port)
			delete(r.listenedPort, port)
		}

		if len(config.MulticastIP) > 0 {
			err := syscall.SetsockoptIPMreq(r.multicastConfigs[config.MulticastIP].Fd,
				syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, r.multicastConfigs[config.MulticastIP].IPMreq)
			if err != nil {
				fmt.Println("离开组播成功 " + config.MulticastIP)
			} else {
				fmt.Println("离开组播失败 " + config.MulticastIP)
			}
			delete(r.joinedMulticastGroup, config.MulticastIP)
			delete(r.multicastConfigs, config.MulticastIP)
		}

		delete(r.configs, id)
	}
}

func (r *udpSourceRecordStruct) DeleteAll() {
	// 停止所有协程
	r.mu.Lock()
	for _, cancel := range r.cancels {
		cancel()
	}
	r.mu.Unlock()

	// 等待所有协程结束
	r.wg.Wait()

	// 离开组播
	for _, v := range r.multicastConfigs {
		syscall.SetsockoptIPMreq(v.Fd, syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, v.IPMreq)
	}
	// 断开连接
	for _, v := range r.conns {
		v.Close()
	}
	r.conns = make(map[uint16]*net.UDPConn)
	r.configs = make(map[string]*common.DataSourceUDPAddrStruct)
	r.listenedPort = make(map[uint16]uint8)
	r.joinedMulticastGroup = make(map[string]uint8)
	r.multicastConfigs = make(map[string]*multicastConfig)
	r.contexts = make(map[string]context.Context)
	r.cancels = make(map[string]context.CancelFunc)
}

type mqttSourceRecordStruct struct {
	conns   map[string]mqtt.Client
	configs map[string]*common.DataSourceMQTTAddrStruct
	client  map[string]map[string]uint8 // key: broker value: clientID

	// 协程管理
	contexts    map[string]context.Context    // key 是 id，用于控制协程退出
	cancels     map[string]context.CancelFunc // key 是 id，用于取消协程
	wg          sync.WaitGroup                // 等待所有协程结束
	broadcaster *DataBroadcaster              // 数据广播器，替代通道
	mu          sync.RWMutex                  // 保护并发访问
}

// receiveMQTTData MQTT消息处理回调
func (r *mqttSourceRecordStruct) receiveMQTTData(id, broker, clientID, topic string) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		// 检查协程是否应该继续运行
		r.mu.RLock()
		ctx, exists := r.contexts[id]
		r.mu.RUnlock()

		if !exists {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			// 创建数据消息
			dataMsg := &DataMessage{
				SourceID: id,
				Data:     make([]byte, len(msg.Payload())),
				From:     topic,
			}
			copy(dataMsg.Data, msg.Payload())

			fmt.Printf("Receive From MQTT, broker:%s, clientID:%s, topic:%s, Size:%d\n",
				broker, clientID, topic, len(msg.Payload()))

			// 广播数据给所有订阅者（如果没有订阅者会自动丢弃）
			r.broadcaster.Broadcast(dataMsg)
		}
	}
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

		// 启动协程管理
		ctx, cancel := context.WithCancel(context.Background())
		r.mu.Lock()
		r.contexts[id] = ctx
		r.cancels[id] = cancel
		r.mu.Unlock()

		// 订阅主题并设置消息处理回调
		for _, t := range addr.MQTT.Topics {
			mqttClient.Subscribe(t, 0, r.receiveMQTTData(id, broker, addr.MQTT.ClientID, t))
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
	// 停止协程
	r.mu.Lock()
	if cancel, exists := r.cancels[id]; exists {
		cancel()
		delete(r.contexts, id)
		delete(r.cancels, id)
	}
	r.mu.Unlock()

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
	// 停止所有协程
	r.mu.Lock()
	for _, cancel := range r.cancels {
		cancel()
	}
	r.mu.Unlock()

	for _, v := range r.conns {
		v.Disconnect(0)
	}
	r.conns = make(map[string]mqtt.Client)
	r.configs = make(map[string]*common.DataSourceMQTTAddrStruct)
	r.client = make(map[string]map[string]uint8)
	r.contexts = make(map[string]context.Context)
	r.cancels = make(map[string]context.CancelFunc)
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
		conns:                make(map[uint16]*net.UDPConn),
		configs:              make(map[string]*common.DataSourceUDPAddrStruct),
		listenedPort:         make(map[uint16]uint8),
		joinedMulticastGroup: make(map[string]uint8),
		multicastConfigs:     make(map[string]*multicastConfig),
		contexts:             make(map[string]context.Context),
		cancels:              make(map[string]context.CancelFunc),
		broadcaster:          NewDataBroadcaster(),
	}
	mqttSourceRecord = &mqttSourceRecordStruct{
		conns:       make(map[string]mqtt.Client),
		configs:     make(map[string]*common.DataSourceMQTTAddrStruct),
		client:      make(map[string]map[string]uint8),
		contexts:    make(map[string]context.Context),
		cancels:     make(map[string]context.CancelFunc),
		broadcaster: NewDataBroadcaster(),
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

// SubscribeUDPData 订阅UDP数据（并发处理），返回取消订阅的函数
func SubscribeUDPData(handlerID string, handler DataHandler) func() {
	return udpSourceRecord.broadcaster.Subscribe(handlerID, handler)
}

// SubscribeUDPDataSequential 订阅UDP数据（顺序处理），返回取消订阅的函数
func SubscribeUDPDataSequential(handlerID string, handler DataHandler, bufferSize int) func() {
	return udpSourceRecord.broadcaster.SubscribeSequential(handlerID, handler, bufferSize)
}

// SubscribeMQTTData 订阅MQTT数据（并发处理），返回取消订阅的函数
func SubscribeMQTTData(handlerID string, handler DataHandler) func() {
	return mqttSourceRecord.broadcaster.Subscribe(handlerID, handler)
}

// SubscribeMQTTDataSequential 订阅MQTT数据（顺序处理），返回取消订阅的函数
func SubscribeMQTTDataSequential(handlerID string, handler DataHandler, bufferSize int) func() {
	return mqttSourceRecord.broadcaster.SubscribeSequential(handlerID, handler, bufferSize)
}

// SubscribeAllData 订阅所有数据源的数据（并发处理），返回取消订阅的函数
func SubscribeAllData(handlerID string, handler DataHandler) func() {
	unsubscribeUDP := SubscribeUDPData(handlerID+"_udp", handler)
	unsubscribeMQTT := SubscribeMQTTData(handlerID+"_mqtt", handler)

	// 返回取消所有订阅的函数
	return func() {
		unsubscribeUDP()
		unsubscribeMQTT()
	}
}

// SubscribeAllDataSequential 订阅所有数据源的数据（顺序处理），返回取消订阅的函数
func SubscribeAllDataSequential(handlerID string, handler DataHandler, bufferSize int) func() {
	unsubscribeUDP := SubscribeUDPDataSequential(handlerID+"_udp", handler, bufferSize)
	unsubscribeMQTT := SubscribeMQTTDataSequential(handlerID+"_mqtt", handler, bufferSize)

	// 返回取消所有订阅的函数
	return func() {
		unsubscribeUDP()
		unsubscribeMQTT()
	}
}

// SubscribeWithConfig 使用自定义配置订阅数据
func SubscribeWithConfig(handlerID string, config HandlerConfig, dataSource string) func() {
	switch dataSource {
	case "UDP":
		return udpSourceRecord.broadcaster.SubscribeWithConfig(handlerID, config)
	case "MQTT":
		return mqttSourceRecord.broadcaster.SubscribeWithConfig(handlerID, config)
	case "ALL":
		unsubscribeUDP := udpSourceRecord.broadcaster.SubscribeWithConfig(handlerID+"_udp", config)
		unsubscribeMQTT := mqttSourceRecord.broadcaster.SubscribeWithConfig(handlerID+"_mqtt", config)
		return func() {
			unsubscribeUDP()
			unsubscribeMQTT()
		}
	default:
		return func() {} // 无效的数据源
	}
}

// GetDataHandlerCount 获取数据处理者数量
func GetDataHandlerCount() (udpCount, mqttCount int) {
	return udpSourceRecord.broadcaster.GetHandlerCount(),
		mqttSourceRecord.broadcaster.GetHandlerCount()
}

// SetDataProcessor 设置数据处理器（兼容旧接口）
func SetDataProcessor(processorID string, processor DataProcessor) func() {
	handler := func(msg *DataMessage) {
		processor.ProcessData(msg)
	}
	return SubscribeAllData(processorID, handler)
}
