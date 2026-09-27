// Package health 提供有时限约束的 Nexo 服务就绪探测。
package health

import (
	"context"
	"errors"
)

// Probe 检查单项依赖；探测函数应响应传入的 context 取消信号。
type Probe func(context.Context) error

// Checker 并发执行依赖探测，避免某个依赖响应较慢时逐项等待、拉长总检查时间。
type Checker struct {
	probes map[string]Probe
}

// Report 可安全返回给就绪检查接口：只报告状态，不暴露连接串或底层错误细节。
type Report struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

// NewChecker 复制探测项映射，避免调用方在服务启动后修改检查清单。
func NewChecker(probes map[string]Probe) *Checker {
	copyOfProbes := make(map[string]Probe, len(probes))
	for name, probe := range probes {
		copyOfProbes[name] = probe
	}
	return &Checker{probes: copyOfProbes}
}

// Check 并发执行全部依赖探测，并将失败或超时的依赖标记为不可用；总检查时限由调用方传入的 ctx 控制。
func (c *Checker) Check(ctx context.Context) Report {
	if c == nil || len(c.probes) == 0 {
		return Report{Status: "not_ready", Dependencies: map[string]string{}}
	}

	type probeResult struct {
		name string
		err  error
	}
	results := make(chan probeResult, len(c.probes))
	for name, probe := range c.probes {
		go func(name string, probe Probe) {
			if probe == nil {
				results <- probeResult{name: name, err: errors.New("probe is nil")}
				return
			}
			results <- probeResult{name: name, err: probe(ctx)}
		}(name, probe)
	}

	dependencies := make(map[string]string, len(c.probes))
	for completed := 0; completed < len(c.probes); {
		select {
		case result := <-results:
			completed++
			if result.err == nil {
				dependencies[result.name] = "ok"
			} else {
				dependencies[result.name] = "unavailable"
			}
		case <-ctx.Done():
			for name := range c.probes {
				if _, finished := dependencies[name]; !finished {
					dependencies[name] = "unavailable"
				}
			}
			return makeReport(dependencies)
		}
	}
	return makeReport(dependencies)
}

func makeReport(dependencies map[string]string) Report {
	for _, status := range dependencies {
		if status != "ok" {
			return Report{Status: "not_ready", Dependencies: dependencies}
		}
	}
	return Report{Status: "ready", Dependencies: dependencies}
}
