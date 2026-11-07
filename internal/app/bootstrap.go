package app

import (
	"errors"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/httpserver"
	"github.com/dujiao-next/internal/app/jobs"
	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
)

// shouldStartWorker 决定是否启动异步消费者。
//
// 队列是可选依赖：没有 Redis 的开发环境把它关掉，也应该能把 API 跑起来，
// 而不是让整个进程退出。worker 模式则是显式要求"只跑消费者"，这时队列缺失
// 属于配置错误，交给 jobs.NewService 报错——否则进程会静默空转，什么都不消费。
//
// 抽成独立函数是为了让这条规则能被单测覆盖，而不必为此拉起整个容器和数据库。
func shouldStartWorker(mode string, queueEnabled bool) bool {
	return mode == ModeWorker || (mode == ModeAll && queueEnabled)
}

// BuildRunner 构建服务运行器
func BuildRunner(cfg *config.Config, mode string) (*Runner, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	dependencies, err := container.NewContainer(cfg)
	if err != nil {
		return nil, err
	}

	var services []Service

	// 初始化 HTTP 服务
	if mode == ModeAll || mode == ModeAPI {
		engine := httpserver.SetupRouter(cfg, dependencies)
		addr := cfg.Server.Host + ":" + cfg.Server.Port
		httpService := NewHTTPService(addr, engine)
		services = append(services, httpService)
		if dependencies.VisitCollector != nil {
			services = append(services, newAnalyticsService(dependencies.VisitCollector))
		}
	}

	// 初始化 Worker 服务
	if shouldStartWorker(mode, cfg.Queue.Enabled) {
		consumer := jobconsumer.New(dependencies)
		workerService, err := jobs.NewService(&cfg.Queue, consumer)
		if err != nil {
			return nil, err
		}
		services = append(services, workerService)
	} else if mode == ModeAll {
		logger.Warnw("queue_disabled_worker_skipped",
			"hint", "依赖异步队列的功能（下单、邮件、上游同步等）不可用；需要时请在 config.yml 开启 redis 与 queue")
	}

	// 如果没有服务被启动（例如模式错误或配置导致都没起），应该报错或至少打日志
	if len(services) == 0 {
		return nil, errors.New("no services initialized (check mode and config)")
	}

	return NewRunner(services...), nil
}

// Run 应用启动入口
func Run(opts Options) error {
	opts = normalizeOptions(opts)
	if opts.Config == nil {
		return errors.New("config is nil")
	}

	runner, err := BuildRunner(opts.Config, opts.Mode)
	if err != nil {
		return err
	}

	addr := opts.Config.Server.Host + ":" + opts.Config.Server.Port
	opts.Logger.Infow("app_start", "addr", addr, "mode", opts.Mode)
	return RunWithOptions(runner, opts)
}
