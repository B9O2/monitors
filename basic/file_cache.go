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

type FileCacheMonitor[T any, R any] struct {
	mt *Multitasking.Multitasking[T, R]
	ms *core.MonitorServer[T, R]
}

func (fcm *FileCacheMonitor[T, R]) Start(
	ctx context.Context,
	addr string,
	threads uint64,
	credential credentials.TransportCredentials,
) (result []R, err error) {

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

func NewFileCacheMonitor[T, R any](
	mt *Multitasking.Multitasking[T, R],
	fileName string,
	maxSize int,
	maxAge int,
) (*FileCacheMonitor[T, R], error) {
	writer := &lumberjack.Logger{
		Filename:   fileName,
		MaxSize:    maxSize, // MB
		MaxBackups: 1,
		MaxAge:     maxAge,
		Compress:   false,
	}

	mt.SetLogger(func(l zerolog.Logger) zerolog.Logger {
		zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
		return l.Output(writer).
			With().
			Timestamp().
			Logger()
	})

	ms, err := core.NewMonitorServer(mt)
	if err != nil {
		return nil, err
	}

	ms.SetLogReader(
		func(theadID int64, skipLine uint64, after time.Time) []string {
			res, err := utils.ReadFromLine(fileName, int(skipLine))
			if err != nil {
				return []string{"Log Reader Error:" + err.Error()}
			}
			return res
		},
	)

	return &FileCacheMonitor[T, R]{
		mt: mt,
		ms: ms,
	}, nil
}
