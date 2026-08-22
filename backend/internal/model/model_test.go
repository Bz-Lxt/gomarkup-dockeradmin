package model

import "testing"

func validRule() AlertRule {
	return AlertRule{
		Name:        "CPU 过高",
		Metric:      MetricCPUPercent,
		Op:          OpGt,
		Threshold:   80,
		DurationSec: 30,
		CooldownSec: 300,
		Enabled:     true,
		WebhookURL:  "http://localhost:8080/api/mock/webhook",
	}
}

func TestValidateRule_OK(t *testing.T) {
	r := validRule()
	if errs := ValidateRule(&r); len(errs) > 0 {
		t.Fatalf("合法规则校验失败: %v", errs)
	}
}

func TestValidateRule_Fields(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*AlertRule)
		wantField string
	}{
		{"名称为空", func(r *AlertRule) { r.Name = " " }, "name"},
		{"未知指标", func(r *AlertRule) { r.Metric = "bogus" }, "metric"},
		{"未知运算符", func(r *AlertRule) { r.Op = "!" }, "op"},
		{"阈值超界", func(r *AlertRule) { r.Threshold = 101 }, "threshold"},
		{"持续时间为负", func(r *AlertRule) { r.DurationSec = -1 }, "duration_sec"},
		{"冷却期超界", func(r *AlertRule) { r.CooldownSec = 86401 }, "cooldown_sec"},
		{"webhook 为空", func(r *AlertRule) { r.WebhookURL = "" }, "webhook_url"},
		{"webhook 非 http", func(r *AlertRule) { r.WebhookURL = "ftp://x.com/hook" }, "webhook_url"},
		{"webhook 无 host", func(r *AlertRule) { r.WebhookURL = "http://" }, "webhook_url"},
		{"容器指标缺 target", func(r *AlertRule) {
			r.Metric = MetricContainerCPUPct
			r.Target = ""
		}, "target"},
		{"容器 CPU 阈值可超 100", func(r *AlertRule) {
			r.Metric = MetricContainerCPUPct
			r.Target = "web"
			r.Threshold = 500
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validRule()
			tc.mutate(&r)
			errs := ValidateRule(&r)
			if tc.wantField == "" {
				if len(errs) > 0 {
					t.Fatalf("期望通过，实际错误: %v", errs)
				}
				return
			}
			found := false
			for _, e := range errs {
				if e.Field == tc.wantField {
					found = true
				}
			}
			if !found {
				t.Fatalf("期望字段 %q 报错，实际: %v", tc.wantField, errs)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	if !Compare(OpGt, 81, 80) || Compare(OpGt, 80, 80) {
		t.Fatal("OpGt 语义错误")
	}
	if !Compare(OpGe, 80, 80) {
		t.Fatal("OpGe 语义错误")
	}
	if !Compare(OpLt, 79, 80) || Compare(OpLt, 80, 80) {
		t.Fatal("OpLt 语义错误")
	}
	if !Compare(OpLe, 80, 80) {
		t.Fatal("OpLe 语义错误")
	}
}
