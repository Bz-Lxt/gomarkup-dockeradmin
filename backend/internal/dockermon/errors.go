package dockermon

import "errors"

// ErrUnavailable docker.sock 不可用（降级模式）
var ErrUnavailable = errors.New("docker unavailable: running in degraded mode")
