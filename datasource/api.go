package datasource

import (
	"encoding/json"
	"net"
	"smdp-gateway/common"
	"smdp-gateway/logger"
	"smdp-gateway/utils"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

func checkDataSourceName(ctx iris.Context, name string) bool {
	if len(name) == 0 {
		errCode := common.ErrCodeDataSourceNameEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceName " + common.CodeMessage[errCode])
		return false
	}
	if utils.CalcUtf8StringLen(name) > 32 {
		errCode := common.ErrCodeDataSourceNameTooLong
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceName " + common.CodeMessage[errCode])
		return false
	}
	return true
}

func checkDataSourceDescription(ctx iris.Context, desc string) bool {
	if utils.CalcUtf8StringLen(desc) > 256 {
		errCode := common.ErrCodeDataSourceDescriptionTooLong
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceDescription " + common.CodeMessage[errCode])
		return false
	}
	return true
}

func checkDataSourceType(ctx iris.Context, t string) bool {
	if _, ok := validDataSourceType[t]; !ok {
		errCode := common.ErrCodeDataSourceTypeInvalid
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceType " + common.CodeMessage[errCode])
		return false
	}
	return true
}

func checkDataSourceAddr(ctx iris.Context, t, addr string) bool {
	switch t {
	case "UDP":
		return checkDataSourceAddrWhenUDP(ctx, addr)
	case "MQTT":
		return checkDataSourceAddrWhenMQTT(ctx, addr)
	}

	return false
}

func checkDataSourceAddrWhenUDP(ctx iris.Context, addr string) bool {
	if len(addr) == 0 {
		errCode := common.ErrCodeDataSourceUDPAddrEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
		return false
	}
	// 纯端口
	if port, err := strconv.Atoi(addr); err == nil {
		if port < 1 || port > 65535 {
			errCode := common.ErrCodeDataSourceUDPAddrInvalidPort
			common.ReturnBadRequest(ctx, errCode)
			logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
			return false
		}
		return true
	}
	// 组播地址加端口
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		errCode := common.ErrCodeDataSourceUDPAddrInvalidPort
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
		return false
	}
	ipStr, portStr := parts[0], parts[1]
	ip := net.ParseIP(ipStr)
	if ip == nil {
		errCode := common.ErrCodeDataSourceUDPAddrInvalidPort
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
		return false
	}
	if !utils.IsMulticastIP(ip) { // 校验是否为组播地址
		errCode := common.ErrCodeDataSourceUDPAddrInvalidIP
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
		return false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		errCode := common.ErrCodeDataSourceUDPAddrInvalidPort
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenUDP " + common.CodeMessage[errCode])
		return false
	}
	return true
}

func checkDataSourceAddrWhenMQTT(ctx iris.Context, addr string) bool {
	if len(addr) == 0 {
		errCode := common.ErrCodeDataSourceMQTTAddrEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenMQTT " + common.CodeMessage[errCode])
		return false
	}
	if len(addr) > 2048 {
		errCode := common.ErrCodeDataSourceMQTTAddrTooLong
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("checkDataSourceAddrWhenMQTT " + common.CodeMessage[errCode])
		return false
	}
	return true
}

// AddDataSource 添加数据源
func AddDataSource(ctx iris.Context) {
	logger.Default(ctx).Info("AddDataSource", zap.String("ip", ctx.RemoteAddr()))

	bodyBytes, _ := ctx.GetBody()
	req := model{}
	err := json.Unmarshal(bodyBytes, &req)
	if err != nil {
		common.ReturnParseBodyError(ctx)
		logger.Default(ctx).Error("AddDataSource 反序列化body失败", zap.Error(err))
		return
	}

	// 首先检查必要参数
	if !checkDataSourceName(ctx, req.Name) || !checkDataSourceDescription(ctx, *req.Description) ||
		!checkDataSourceType(ctx, req.Type) || !checkDataSourceAddr(ctx, req.Type, req.Addr) {
		return
	}

	// 检查是否有重名
	var pageNumber int = 1
	var pageSize int = 10
	items, err := selectByName(-1, req.Name, false, &pageNumber, &pageSize)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("AddDataSource 根据Name查询数据库失败", zap.Error(err))
		return
	}
	if len(items) > 0 {
		errCode := common.ErrCodeDataSourceNameDuplicate
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("AddDataSource " + common.CodeMessage[errCode])
		return
	}
	// 检查type+addr是否有重复
	items, err = selectByConfig(req.Type, req.Addr)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("AddDataSource 根据Type+Addr查询数据库失败", zap.Error(err))
		return
	}
	if len(items) > 0 {
		errCode := common.ErrCodeDataSourceConfigDuplicate
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("AddDataSource " + common.CodeMessage[errCode])
		return
	}
	// 插入数据
	err = insert(&req)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("AddDataSource 插入数据库失败", zap.Error(err))
		return
	}

	req.State = 0
	common.ReturnOK(ctx, req)
}

// DeleteDatasource 删除数据源
func DeleteDatasource(ctx iris.Context) {
	logger.Default(ctx).Info("DeleteDatasource interface", zap.String("ip", ctx.RemoteAddr()))

	id := ctx.URLParam("id")
	if len(id) == 0 {
		errCode := common.ErrCodeDataSourceIDEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("DeleteDatasource " + common.CodeMessage[errCode])
		return
	}
	// 检查是否存在
	items, err := selectByID(id)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("DeleteDatasource 根据ID查询数据库失败", zap.Error(err))
		return
	}
	if len(items) == 0 {
		errCode := common.ErrCodeDataSourceIDNotFound
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("DeleteDatasource " + common.CodeMessage[errCode])
		return
	}
	// 判断状态
	if items[0].State != 0 {
		errCode := common.ErrCodeDataSourceStateOpened
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("DeleteDatasource " + common.CodeMessage[errCode])
		return
	}

	deleteByID(id)

	common.ReturnOK(ctx, nil)
}

// UpdateDataSourceConfig 更新数据源配置
func UpdateDataSourceConfig(ctx iris.Context) {
	logger.Default(ctx).Info("UpdateDataSourceConfig", zap.String("ip", ctx.RemoteAddr()))

	id := ctx.URLParam("id")
	if len(id) == 0 {
		errCode := common.ErrCodeDataSourceIDEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceConfig " + common.CodeMessage[errCode])
		return
	}
	// 检查是否存在
	items, err := selectByID(id)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 根据ID查询数据库失败", zap.Error(err))
		return
	}
	if len(items) == 0 {
		errCode := common.ErrCodeDataSourceIDNotFound
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceConfig " + common.CodeMessage[errCode])
		return
	}
	// 判断状态
	if items[0].State != 0 {
		errCode := common.ErrCodeDataSourceStateOpened
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceConfig " + common.CodeMessage[errCode])
		return
	}

	bodyBytes, _ := ctx.GetBody()
	req := model{}
	err = json.Unmarshal(bodyBytes, &req)
	if err != nil {
		common.ReturnParseBodyError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 反序列化body失败", zap.Error(err))
		return
	}

	// 首先检查必要参数
	if !checkDataSourceName(ctx, req.Name) || !checkDataSourceDescription(ctx, *req.Description) ||
		!checkDataSourceType(ctx, req.Type) || !checkDataSourceAddr(ctx, req.Type, req.Addr) {
		return
	}

	// 检查是否有重名
	var pageNumber int = 1
	var pageSize int = 10
	items, err = selectByName(-1, req.Name, false, &pageNumber, &pageSize)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 根据Name查询数据库失败", zap.Error(err))
		return
	}
	if len(items) > 0 {
		errCode := common.ErrCodeDataSourceNameDuplicate
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceConfig " + common.CodeMessage[errCode])
		return
	}
	// 检查type+addr是否有重复
	items, err = selectByConfig(req.Type, req.Addr)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 根据Type+Addr查询数据库失败", zap.Error(err))
		return
	}
	if len(items) > 0 {
		errCode := common.ErrCodeDataSourceConfigDuplicate
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceConfig " + common.CodeMessage[errCode])
		return
	}
	// 更新数据
	err = updateConfig(id, &req)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 更新数据库失败", zap.Error(err))
		return
	}

	items, err = selectByID(id)
	if err != nil || len(items) == 0 {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceConfig 根据ID查询数据库失败", zap.Error(err))
		return
	}

	common.ReturnOK(ctx, items[0])
}

// UpdateDataSourceState 更新数据源状态
func UpdateDataSourceState(ctx iris.Context) {
	logger.Default(ctx).Info("UpdateDataSourceState", zap.String("ip", ctx.RemoteAddr()))

	id := ctx.URLParam("id")
	if len(id) == 0 {
		errCode := common.ErrCodeDataSourceIDEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceState " + common.CodeMessage[errCode])
		return
	}
	// 检查是否存在
	items, err := selectByID(id)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceState 根据ID查询数据库失败", zap.Error(err))
		return
	}
	if len(items) == 0 {
		errCode := common.ErrCodeDataSourceIDNotFound
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("UpdateDataSourceState " + common.CodeMessage[errCode])
		return
	}

	state := 0
	if ctx.URLParam("state") == "1" {
		state = 1
	}

	// 更新数据
	err = updateState(id, state)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("UpdateDataSourceState 更新数据库失败", zap.Error(err))
		return
	}

	if state == 0 {
		// TODO 开启
	} else {
		// TODO 关闭
	}

	common.ReturnOK(ctx, nil)
}

// ListDataSources 获取数据源列表
func ListDataSources(ctx iris.Context) {
	state := ctx.URLParamIntDefault("state", -1)
	name := ctx.URLParam("name")
	pageNumber := ctx.URLParamIntDefault("pageNumber", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 10)

	total, err := count(state, name)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("ListDataSources 获取数据源总数失败", zap.Error(err))
		return
	}

	items, err := selectByName(state, name, true, &pageNumber, &pageSize)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("ListDataSources 获取数据源列表失败", zap.Error(err))
		return
	}

	// TODO 添加数据源运行状态的逻辑
	type modelSimple struct {
		ID        string    `json:"id"`
		State     int       `json:"state"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
		Addr      string    `json:"addr"`
		Utime     time.Time `json:"utime"`
		IsRunning bool      `json:"isRunning"`
		Message   string    `json:"message"`
	}
	var rsp struct {
		Total      int64          `json:"total"`
		PageNumber int            `json:"pageNumber"`
		PageSize   int            `json:"pageSize"`
		List       []*modelSimple `json:"list"`
	}
	if len(items) == 0 {
		rsp.Total = total
		rsp.PageNumber = pageNumber
		rsp.PageSize = pageSize
		rsp.List = []*modelSimple{}
		common.ReturnOK(ctx, rsp)
		return
	}

	for _, value := range items {
		one := &modelSimple{
			ID:    value.ID,
			State: value.State,
			Name:  value.Name,
			Type:  value.Type,
			Addr:  value.Addr,
			Utime: value.Utime,
		}
		rsp.List = append(rsp.List, one)
	}
	rsp.Total = total
	rsp.PageNumber = pageNumber
	rsp.PageSize = pageSize

	common.ReturnOK(ctx, rsp)
}

// GetDataSource 根据获取数据源详情
func GetDataSource(ctx iris.Context) {
	id := ctx.URLParam("id")
	if len(id) == 0 {
		errCode := common.ErrCodeDataSourceIDEmpty
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("GetDataSource " + common.CodeMessage[errCode])
		return
	}
	// 检查是否存在
	items, err := selectByID(id)
	if err != nil {
		common.ReturnInternalError(ctx)
		logger.Default(ctx).Error("GetDataSource 根据ID查询数据库失败", zap.Error(err))
		return
	}
	if len(items) == 0 {
		errCode := common.ErrCodeDataSourceIDNotFound
		common.ReturnBadRequest(ctx, errCode)
		logger.Default(ctx).Error("GetDataSource " + common.CodeMessage[errCode])
		return
	}

	type modelDetail struct {
		model
		IsRunning bool   `json:"isRunning"`
		Message   string `json:"message"`
	}

	// TODO 添加数据源运行状态的逻辑
	var rsp modelDetail
	rsp.model = items[0]

	common.ReturnOK(ctx, rsp)
}
