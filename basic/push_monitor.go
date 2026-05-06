package basic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/B9O2/Multitasking"
	"github.com/B9O2/monitors/core"
	"github.com/B9O2/monitors/monitor"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type MemoryLogBuffer struct {
	logs []string
	mu   sync.Mutex
}

func (m *MemoryLogBuffer) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// zerolog outputs JSON by default, or console-formatted text. 
	// We strip the trailing newline if present to keep the log lines clean.
	line := string(p)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	m.logs = append(m.logs, line)
	return len(p), nil
}

func (m *MemoryLogBuffer) Drain() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logs) == 0 {
		return nil
	}
	logs := m.logs
	m.logs = nil
	return logs
}

type PushMonitor[T any, R any] struct {
	mt *Multitasking.Multitasking[T, R]
}

func NewPushMonitor[T, R any](mt *Multitasking.Multitasking[T, R]) *PushMonitor[T, R] {
	return &PushMonitor[T, R]{
		mt: mt,
	}
}

func (pm *PushMonitor[T, R]) Start(
	ctx context.Context,
	addr string,
	threads uint64,
	interval time.Duration,
	credential credentials.TransportCredentials,
) (result []R, err error) {
	if credential == nil {
		fmt.Println("[!]No credentials provided, using insecure connection")
		credential = insecure.NewCredentials()
	}

	mc, err := core.NewMonitorClient(addr, grpc.WithTransportCredentials(credential))
	if err != nil {
		return nil, err
	}
	defer mc.Close()

	logBuffer := &MemoryLogBuffer{}
	pm.mt.SetLogger(func(l zerolog.Logger) zerolog.Logger {
		zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
		return l.Output(logBuffer).With().Timestamp().Logger()
	})

	// Use a wait group to ensure background pushers finish before closing connection
	var wg sync.WaitGroup
	pushCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Background status push
	wg.Add(1)
	go func() {
		defer wg.Done()
		statusStream, err := mc.PushStatus(pushCtx)
		if err != nil {
			fmt.Printf("[!]Failed to open status push stream: %v\n", err)
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				status := &monitor.Status{
					Name:        pm.mt.Name(),
					TotalTask:   pm.mt.TotalTask(),
					TotalRetry:  pm.mt.TotalRetry(),
					TotalResult: pm.mt.TotalResult(),
					RetrySize:   pm.mt.MaxRetryQueue(),
					ThreadsDetail: &monitor.ThreadsDetail{
						ThreadsStatus: pm.mt.ThreadsDetail().AllStatus(),
						ThreadsCount:  pm.mt.ThreadsDetail().AllCounter(),
					},
					Interval: uint64(interval),
				}
				if err := statusStream.Send(status); err != nil {
					fmt.Printf("[!]Failed to send status: %v\n", err)
					return
				}
			case <-pushCtx.Done():
				statusStream.CloseAndRecv()
				return
			}
		}
	}()

	// Background events push
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventsStream, err := mc.PushEvents(pushCtx)
		if err != nil {
			fmt.Printf("[!]Failed to open events push stream: %v\n", err)
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logs := logBuffer.Drain()
				if len(logs) > 0 {
					if err := eventsStream.Send(&monitor.Events{
						Name: pm.mt.Name(),
						Logs: logs,
					}); err != nil {
						fmt.Printf("[!]Failed to send events: %v\n", err)
						return
					}
				}
			case <-pushCtx.Done():
				// One last drain
				logs := logBuffer.Drain()
				if len(logs) > 0 {
					eventsStream.Send(&monitor.Events{
						Name: pm.mt.Name(),
						Logs: logs,
					})
				}
				eventsStream.CloseAndRecv()
				return
			}
		}
	}()

	result, err = pm.mt.Run(ctx, threads)
	
	// Signal background pushers to stop and wait for them
	cancel()
	wg.Wait()
	
	return result, err
}
