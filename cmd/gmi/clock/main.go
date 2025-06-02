/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/05/27
 * @Desc    : 全局时钟服务
 */

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/karmada-io/karmada/pkg/clock"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/piaobeizu/titan/log"
	"github.com/sirupsen/logrus"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
)

func main() {
	log.InitLog("gmi-clock", util.GetEnv("GMI_LOG_LEVEL", "debug"))
	// 1. create a time service
	ctx, cancel := context.WithCancel(context.Background())
	port := util.GetEnv("GMI_CLOCK_SERVICE_PORT", ":8081")

	timeService := clock.NewTimeServer(ctx, port)
	go func() {
		if err := timeService.Start(); err != nil {
			logrus.Fatal("gmi clock service started failed:", err)
		}
	}()

	// system signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-signalChan
	cancel()
	fmt.Println("gmi time service stopped")
}
