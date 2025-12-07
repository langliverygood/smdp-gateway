package main

import (
	"fmt"
	"os/signal"
	"sync"
	"syscall"

	"os"
	"runtime"
	"runtime/pprof"
	"smdp-gateway/config"
	"smdp-gateway/datasource"
	"smdp-gateway/logger"
	"time"

	"go.uber.org/zap"
)

func main() {
	// 解析配置文件
	if err := config.Parse("./config/main.toml"); err != nil {
		panic(fmt.Sprintf("解析配置文件失败: %s", err.Error()))
	}
	// 初始化日志
	if err := logger.Init(); err != nil {
		panic(fmt.Sprintf("初始化日志失败: %s", err.Error()))
	}
	defer logger.Sync()
	// 打开调试模式
	if config.Global().Server.DebugMode {
		openPprof()
	}

	// 启动iris框架，打开tcp服务
	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	go startIris(&wg, stopChan)

	datasource.Init()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2)
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// 正常关闭流程
			logger.Default().Info("收到终止信号...", zap.String("signal", sig.String()))
			close(stopChan)
			wg.Wait()
			return // 退出程序

		case syscall.SIGHUP, syscall.SIGUSR2:
			// 重新加载插件
			logger.Default().Info("收到重载信号...", zap.String("signal", sig.String()))
		}
	}

}

func openPprof() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			f, err := os.Create("service.pprof")
			if err != nil {
				panic(err.Error())
			}

			runtime.GC() // 获取最新统计信息
			if err := pprof.WriteHeapProfile(f); err != nil {
				logger.Default().Error("could not write memory profile: " + err.Error())
			}

			if err := f.Close(); err != nil {
				logger.Default().Error("could not close profile file: " + err.Error())
			}
		}
	}()
}
