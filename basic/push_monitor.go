package basic

import (
	"bufio"
	"bytes"
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

	// Use a scanner to handle potential multiple lines in a single write call
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			m.logs = append(m.logs, line)
		}
	}
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
		fmt.Println("[PushMonitor] No credentials provided, using insecure connection")
		credential = insecure.NewCredentials()
	}

	mc, err := core.NewMonitorClient(addr, grpc.WithTransportCredentials(credential))
	if err != nil {
		return nil, err
	}
	defer mc.Close()

	pushCtx, pushCancel := context.WithCancel(ctx)
	defer pushCancel()

	statusStream, err := mc.PushStatus(pushCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to open status push stream: %w", err)
	}

	eventsStream, err := mc.PushEvents(pushCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to open events push stream: %w", err)
	}

	logBuffer := &MemoryLogBuffer{}
	pm.mt.SetLogger(func(l zerolog.Logger) zerolog.Logger {
		zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
		// Ensure logs are written to our memory buffer
		return l.Output(logBuffer).With().Timestamp().Logger()
	})

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Background status push
	wg.Add(1)
	go func() {
		defer wg.Done()
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
					fmt.Printf("[PushMonitor] Error sending status: %v\n", err)
					return
				}
			case <-stopChan:
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
				statusStream.Send(status)
				statusStream.CloseAndRecv()
				return
			case <-pushCtx.Done():
				return
			}
		}
	}()

	// Background events push
	wg.Add(1)
	go func() {
		defer wg.Done()
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
						fmt.Printf("[PushMonitor] Error sending events: %v\n", err)
						return
					}
					// fmt.Printf("[PushMonitor] Sent %d logs to %s\n", len(logs), addr)
				}
			case <-stopChan:
				logs := logBuffer.Drain()
				if len(logs) > 0 {
					eventsStream.Send(&monitor.Events{
						Name: pm.mt.Name(),
						Logs: logs,
					})
				}
				eventsStream.CloseAndRecv()
				return
			case <-pushCtx.Done():
				return
			}
		}
	}()

	result, err = pm.mt.Run(ctx, threads)
	
	close(stopChan)
	wg.Wait()
	
	return result, err
}
