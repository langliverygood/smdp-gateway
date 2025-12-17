package common

// CodeMessage 错误码错误信息映射
var CodeMessage map[int]string

var (
	// ErrCodeOK 正常
	ErrCodeOK = 0
	// ErrCodeInternalError 内部错误
	ErrCodeInternalError = -1
	// ErrCodeParseBodyError body解析失败
	ErrCodeParseBodyError = -10

	// ErrCodeInvalidLicense 许可证无效
	ErrCodeInvalidLicense = -11

	// ErrCodeInvalidUsernameOrPassword 用户名或密码无效
	ErrCodeInvalidUsernameOrPassword = -21
	// ErrCodeInvalidTimestamp 时间戳无效
	ErrCodeInvalidTimestamp = -22
	// ErrCodeInvalidToken token无效或已过期
	ErrCodeInvalidToken = -23

	// ErrCodeDataSourceNameEmpty 数据源名称为空
	ErrCodeDataSourceNameEmpty = -10001
	// ErrCodeDataSourceNameDuplicate 数据源名称重复
	ErrCodeDataSourceNameDuplicate = -10002
	// ErrCodeDataSourceTypeEmpty 数据源协议类型为空
	ErrCodeDataSourceTypeEmpty = -10003
	// ErrCodeDataSourceAddrEmpty 数据源协议配置为空
	ErrCodeDataSourceAddrEmpty = -10004
	// ErrCodeDataSourceIDEmpty 数据源协议ID为空
	ErrCodeDataSourceIDEmpty = -10005
	// ErrCodeDataSourceConfigDuplicate 数据源配置重复
	ErrCodeDataSourceConfigDuplicate = -10006
	// ErrCodeDataSourceNameTooLong 数据源名称长度超过限制
	ErrCodeDataSourceNameTooLong = -10007
	// ErrCodeDataSourceDescriptionTooLong 数据源描述长度超过限制
	ErrCodeDataSourceDescriptionTooLong = -10008
	// ErrCodeDataSourceTypeInvalid 数据源类型无效
	ErrCodeDataSourceTypeInvalid = -10009
	// ErrCodeDataSourceUDPAddrEmpty 数据源UDP地址为空
	ErrCodeDataSourceUDPAddrEmpty = -10010
	// ErrCodeDataSourceUDPAddrInvalidIP 数据源UDP地址IP无效,仅支持组播地址
	ErrCodeDataSourceUDPAddrInvalidIP = -10011
	// ErrCodeDataSourceUDPAddrInvalidPort 数据源UDP地址端口无效(端口范围1~65535)
	ErrCodeDataSourceUDPAddrInvalidPort = -10012
	// ErrCodeDataSourceMQTTAddrInvalid 数据源MQTT地址无效
	ErrCodeDataSourceMQTTAddrInvalid = -10013
	// ErrCodeDataSourceMQTTAddrBrokerInvalid 数据源MQTT Broker无效，仅支持IP:端口
	ErrCodeDataSourceMQTTAddrBrokerInvalid = -10014
	// ErrCodeDataSourceMQTTAddrTopicTooLong 数据源MQTT主题长度超过限制
	ErrCodeDataSourceMQTTAddrTopicTooLong = -10015
	// ErrCodeDataSourceMQTTAddrTopicInvalid 数据源MQTT主题格式无效
	ErrCodeDataSourceMQTTAddrTopicInvalid = -10016
	// ErrCodeDataSourceIDNotFound 数据源协议ID不存在
	ErrCodeDataSourceIDNotFound = -10017
	// ErrCodeDataSourceStateOpened 数据源为开启状态,无法更新或删除
	ErrCodeDataSourceStateOpened = -10018
)

func init() {
	CodeMessage = make(map[int]string)

	CodeMessage[ErrCodeOK] = "OK"
	CodeMessage[ErrCodeInternalError] = "内部错误"
	CodeMessage[ErrCodeParseBodyError] = "解析请求失败, 请检查你的请求内容"

	CodeMessage[ErrCodeInvalidUsernameOrPassword] = "认证失败, 用户名或密码错误"
	CodeMessage[ErrCodeInvalidTimestamp] = "认证失败, 时间戳无效"
	CodeMessage[ErrCodeInvalidToken] = "token无效或已过期"

	CodeMessage[ErrCodeInvalidLicense] = "许可证无效"

	CodeMessage[ErrCodeDataSourceNameEmpty] = "数据源名称为空"
	CodeMessage[ErrCodeDataSourceNameDuplicate] = "数据源名称重复"
	CodeMessage[ErrCodeDataSourceTypeEmpty] = "数据源协议类型为空"
	CodeMessage[ErrCodeDataSourceAddrEmpty] = "数据源协议配置为空"
	CodeMessage[ErrCodeDataSourceIDEmpty] = "数据源协议配置为空"
	CodeMessage[ErrCodeDataSourceConfigDuplicate] = "数据源协议配置重复"
	CodeMessage[ErrCodeDataSourceNameTooLong] = "数据源名称长度超过限制"
	CodeMessage[ErrCodeDataSourceDescriptionTooLong] = "数据源描述长度超过限制"
	CodeMessage[ErrCodeDataSourceTypeInvalid] = "数据源类型无效"
	CodeMessage[ErrCodeDataSourceUDPAddrEmpty] = "数据源UDP地址为空"
	CodeMessage[ErrCodeDataSourceUDPAddrInvalidIP] = "数据源UDP地址IP无效,仅支持组播地址"
	CodeMessage[ErrCodeDataSourceUDPAddrInvalidPort] = "数据源UDP地址端口无效(端口范围1~65535)"
	CodeMessage[ErrCodeDataSourceMQTTAddrInvalid] = "数据源MQTT地址无效"
	CodeMessage[ErrCodeDataSourceMQTTAddrBrokerInvalid] = "数据源MQTT Broker无效, 仅支持IP:端口"
	CodeMessage[ErrCodeDataSourceMQTTAddrTopicTooLong] = "数据源MQTT主题长度超过限制"
	CodeMessage[ErrCodeDataSourceMQTTAddrTopicInvalid] = "数据源MQTT主题格式无效"
	CodeMessage[ErrCodeDataSourceIDNotFound] = "数据源协议ID不存在"
	CodeMessage[ErrCodeDataSourceStateOpened] = "数据源为开启状态,无法更新或删除"
}
