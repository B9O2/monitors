package basic

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/B9O2/Multitasking"
	"github.com/rs/zerolog"
)

type Task struct {
	A, B, I int
}

func GenNumbers(dc Multitasking.DistributeController) {
	//dc.Debug(true)
	final := 0
	for i := 0; i < 1000; i++ {
		dc.AddTask(Task{
			A: rand.Int(),
			B: rand.Int(),
			I: i,
		})
		final += 1
	}
}

var CertDir = os.Getenv("CREDS_DIR")

func TestMonitor(t *testing.T) {
	if len(CertDir) <= 0 {
		fmt.Println("Please Set CREDS_DIR")
		return
	}

	mt := Multitasking.NewMultitasking("TestPool", nil)
	mt.Register(
		GenNumbers,
		func(ec Multitasking.ExecuteController, logger zerolog.Logger, a any) any {
			task := a.(Task)
			t := time.Duration(task.A%4) * time.Duration(time.Second)
			//fmt.Printf("> Sleep %s\n",t)
			logger.Info().Str("sleep", t.String()).Msg(fmt.Sprintf("执行任务 Sleep %s", t))
			time.Sleep(t)
			return task.A + task.B
		},
	)

	//初始化FileCacheMonitor
	fcm, err := NewFileCacheMonitor(mt, "test.log", 4, 7)
	if err != nil {
		fmt.Printf("Error creating FileCacheMonitor: %s\n", err)
		return
	}

	//加载证书
	// cert, err := credentials.NewServerTLSFromFile(path.Join(CertDir, "server.crt"), path.Join(CertDir, "server.key"))
	// if err != nil {
	// 	log.Fatalf("failed to load cert: %v", err)
	// }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("Running")
	r, err := fcm.Start(ctx, ":50051", 20, nil)
	if err != nil {
		fmt.Printf("Pool Run Error: %s\n", err)
		return
	}

	fmt.Printf("Total: %d\n", len(r))

	// creds, err := credentials.NewClientTLSFromFile(path.Join(CertDir, "server.crt"), "localhost")
	// if err != nil {
	// 	log.Fatalf("could not load tls cert: %s", err)
	// }

	// client, err := Multitasking.NewMonitorClient("127.0.0.1:50051", grpc.WithTransportCredentials(creds))
	// if err != nil {
	// 	fmt.Printf("Monitor Client Error: %s\n", err)
	// 	return
	// }

	// fmt.Println("[Start Stream Metrics]")
	// stream, err := client.StreamMetrics(ctx, 2*time.Second)
	// if err != nil {
	// 	fmt.Printf("Monitor Client Stream Error: %s\n", err)
	// 	return
	// }

	// for {
	// 	//fmt.Println("Reading...")
	// 	m, err := stream.Receive()
	// 	if err != nil {
	// 		if status.Code(err) != codes.Canceled {
	// 			fmt.Printf("Metrics Error: %s\n", err)
	// 		} else {
	// 			fmt.Printf("[Metrics Stream Closed]\n")
	// 		}
	// 		return
	// 	}
	// 	fmt.Printf("> Metrics %s\n", m)
	// }
}
