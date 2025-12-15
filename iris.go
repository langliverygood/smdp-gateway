package main

import (
	"context"
	"runtime"
	"smdp-gateway/common"
	"smdp-gateway/config"
	"smdp-gateway/datasource"
	"smdp-gateway/logger"
	"sync"
	"time"

	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

func registerRouter(app *iris.Application) {
	app.Post("/api/datasource", common.Preprocessing, common.AdminAuth, datasource.AddDataSource)
	app.Delete("/api/datasource", common.Preprocessing, common.AdminAuth, datasource.DeleteDatasource)
	app.Put("/api/datasource", common.Preprocessing, common.AdminAuth, datasource.UpdateDataSourceConfig)
	app.Put("/api/datasource/state", common.Preprocessing, common.AdminAuth, datasource.UpdateDataSourceState)
	app.Get("/api/datasource/list", common.Preprocessing, common.AdminAuth, datasource.ListDataSources)
	app.Get("/api/datasource", common.Preprocessing, common.AdminAuth, datasource.GetDataSource)
}

func startIris(wg *sync.WaitGroup, stopChan <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// api.CheckToken()
			}
		}
	}()

	wg.Add(1)
	defer wg.Done()

	app := iris.New()
	app.Configure(iris.WithOptimizations, iris.WithoutStartupLog)
	app.Use(recoverHandler)

	// 注册接口
	registerRouter(app)

	irisClosed := make(chan struct{})
	go func() {
		logger.Default().Info("Iris服务启动成功", zap.String("addr", config.Global().Server.AdminAddr))
		err := app.Run(
			iris.Addr(config.Global().Server.AdminAddr),
			iris.WithoutInterruptHandler,                  // 禁用 Iris 默认的 Ctrl+C 处理，由我们自己管理
			iris.WithoutServerError(iris.ErrServerClosed), // 忽略"服务器已关闭"错误
		)
		if err != nil {
			logger.Default().Error("Iris服务启动成功失败", zap.Error(err))
		}
		close(irisClosed)
	}()

	select {
	case <-stopChan:
		logger.Default().Info("接收到关闭信号, 正在关闭Iris服务...")
		// 关闭Iris应用
		if err := app.Shutdown(context.TODO()); err != nil {
			logger.Default().Error("关闭Iris服务器时出错", zap.Error(err))
		} else {
			logger.Default().Info("Iris服务器已优雅关闭")
		}
		return
	case <-irisClosed:
		logger.Default().Error("Iris服务器意外关闭")
	}
}

func recoverHandler(ctx iris.Context) {
	defer func() {
		if err := recover(); err != nil {
			if ctx.IsStopped() {
				return
			}

			// 获取调用栈信息
			buf := make([]byte, 4096)
			length := runtime.Stack(buf, false)
			stacktrace := string(buf[:length])

			// 获取traceID
			traceID := ctx.Values().GetString("traceID")

			// 记录错误日志
			logger.Default().Error("Recovered from panic", zap.String("handler", ctx.HandlerName()), zap.String("traceID", traceID),
				zap.Any("error", err), zap.String("stack", stacktrace), zap.String("method", ctx.Method()),
				zap.String("path", ctx.Path()),
			)

			// 响应客户端
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"code":    500,
				"message": "Internal Server Error",
				"traceID": traceID,
			})
			ctx.StopExecution()
		}
	}()

	ctx.Next()
}
