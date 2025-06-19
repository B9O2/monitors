package basic

import (
	"context"
	"fmt"
	"time"

	"github.com/B9O2/Multitasking"
	"github.com/B9O2/monitors/core"
	"github.com/B9O2/monitors/utils"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type FileCacheMonitor struct {
	mt *Multitasking.Multitasking
	ms *core.MonitorServer
}

func (fcm *FileCacheMonitor) Start(ctx context.Context, addr string, threads uint64, credential credentials.TransportCredentials) (result []any, err error) {

	if credential == nil {
		fmt.Println("[!]No credentials provided, using insecure connection")
		credential = insecure.NewCredentials() // Use insecure connection
	}

	go func() {
		err = core.StartMonitoringServer(addr, fcm.ms, grpc.Creds(credential))
		if err != nil {
			return
		}
	}()

	return fcm.mt.Run(ctx, threads)

}

func NewFileCacheMonitor(mt *Multitasking.Multitasking, fileName string, maxSize int, maxAge int) (*FileCacheMonitor, error) {
	writer := &lumberjack.Logger{
		Filename:   fileName,
		MaxSize:    maxSize, // MB
		MaxBackups: 1,
		MaxAge:     maxAge,
		Compress:   false,
	}

	mt.SetLogger(func(u uint64, l zerolog.Logger) zerolog.Logger {
		zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
		return l.Output(writer).With().Timestamp().Uint64("thread_id", u).Logger()
	})

	ms, err := core.NewMonitorServer(mt)
	if err != nil {
		return nil, err
	}

	ms.SetLogReader(func(theadID int64, skipLine uint64, after time.Time) []string {
		res, err := utils.ReadFromLine(fileName, int(skipLine))
		if err != nil {
			return []string{"Log Reader Error:" + err.Error()}
		}
		return res
	})

	return &FileCacheMonitor{
		mt: mt,
		ms: ms,
	}, nil
}
