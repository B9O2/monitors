package main

import (
	"context"
	"fmt"
	"time"

	"github.com/B9O2/Multitasking"
	"github.com/B9O2/monitors/basic"
)

func main() {
	// 1. 创建一个模拟的任务框架实例
	mt := Multitasking.NewMultitasking[int, string]("Push-Demo-Worker", nil)

	// 2. 注册分发器和执行器
	mt.Register(
		func(dc Multitasking.DistributeController[int, string]) {
			for i := 1; i <= 200; i++ {
				dc.AddTask(i)
				time.Sleep(100 * time.Millisecond)
			}
		},
		func(ec Multitasking.ExecuteController[int, string], tc Multitasking.ThreadController, task int) Multitasking.Result[int, string] {
			time.Sleep(time.Duration(200+task%300) * time.Millisecond)

			logger := tc.Logger()
			if task%5 == 0 {
				logger.Info().
					Msgf("Successfully processed bundle task: %d", task)
			}
			if task%13 == 0 {
				logger.Warn().Msgf("Suspicious task detected: %d", task)
			}

			return ec.Success(fmt.Sprintf("Result-%d", task))
		},
	)

	// 3. 初始化推送监控器
	pm := basic.NewPushMonitor(mt)

	// 4. 启动推送并开始执行任务
	fmt.Println("[Client] Connecting to mtmonitor at localhost:1004...")
	fmt.Println("[Client] Pushing metrics and logs every 1 second...")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	results, err := pm.Start(ctx, "localhost:1004", 10, 1*time.Second, nil)
	if err != nil {
		fmt.Printf("[Client] Error: %v\n", err)
		return
	}

	fmt.Printf(
		"[Client] Task completed. Total results collected: %d\n",
		len(results),
	)
}
