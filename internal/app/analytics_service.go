package app

import (
	"context"
	"errors"

	analyticsapp "github.com/dujiao-next/internal/modules/analytics/application"
)

// analyticsService 把访问采集器的生命周期挂到服务运行器上。
//
// 采集器把 PV 先聚合在内存里，进程退出前必须刷一次盘，否则每次重启/发版都会
// 丢掉最后一个窗口的浏览量。
type analyticsService struct {
	collector *analyticsapp.Collector
}

func newAnalyticsService(collector *analyticsapp.Collector) *analyticsService {
	return &analyticsService{collector: collector}
}

// Name 服务名称
func (s *analyticsService) Name() string {
	return "analytics"
}

// Start 启动采集器并等待关停信号
func (s *analyticsService) Start(ctx context.Context) error {
	if s == nil || s.collector == nil {
		return errors.New("analytics collector not initialized")
	}
	s.collector.Start()
	<-ctx.Done()
	return nil
}

// Stop 落盘内存中剩余的增量
func (s *analyticsService) Stop(_ context.Context) error {
	if s == nil || s.collector == nil {
		return nil
	}
	s.collector.Close()
	return nil
}
