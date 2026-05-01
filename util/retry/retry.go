package retry

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	backoffSteps    = 10
	backoffFactor   = 1.25
	backoffDuration = 5
	backoffJitter   = 1.0
)

// Retry 以指数退避方式重试指定的函数。
func Retry(fn wait.ConditionFunc, initialBackoffSec int) error {
	if initialBackoffSec <= 0 {
		initialBackoffSec = backoffDuration
	}
	backoffConfig := wait.Backoff{
		Steps:    backoffSteps,
		Factor:   backoffFactor,
		Duration: time.Duration(initialBackoffSec) * time.Second,
		Jitter:   backoffJitter,
	}
	retryErr := wait.ExponentialBackoff(backoffConfig, fn)
	if retryErr != nil {
		return retryErr
	}
	return nil
}

// Poll 反复执行条件函数，直到其返回 true、返回错误或达到超时时间。
func Poll(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	return wait.Poll(interval, timeout, condition)
}

// PollImmediate 反复执行条件函数，直到其返回 true、返回错误或达到超时时间。
func PollImmediate(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	return wait.PollImmediate(interval, timeout, condition)
}
