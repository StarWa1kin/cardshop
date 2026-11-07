package app

import "testing"

// 队列在 all 模式下是可选的（开发机上常常没有 Redis），
// 在 worker 模式下是必需的（只跑消费者却没有队列，等于进程空转）。
func TestShouldStartWorker(t *testing.T) {
	cases := []struct {
		name         string
		mode         string
		queueEnabled bool
		want         bool
	}{
		{name: "all mode with queue runs the worker", mode: ModeAll, queueEnabled: true, want: true},
		{name: "all mode without queue skips the worker", mode: ModeAll, queueEnabled: false, want: false},
		{name: "worker mode always builds the worker", mode: ModeWorker, queueEnabled: true, want: true},
		{name: "worker mode without queue still builds so it can report the misconfig", mode: ModeWorker, queueEnabled: false, want: true},
		{name: "api mode never builds the worker", mode: ModeAPI, queueEnabled: true, want: false},
		{name: "api mode without queue never builds the worker", mode: ModeAPI, queueEnabled: false, want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldStartWorker(testCase.mode, testCase.queueEnabled); got != testCase.want {
				t.Fatalf("shouldStartWorker(%q, %v) = %v, want %v",
					testCase.mode, testCase.queueEnabled, got, testCase.want)
			}
		})
	}
}
