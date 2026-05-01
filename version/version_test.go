package version

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	// 调用 Get 函数获取版本信息
	info := Get()

	// 断言返回的 Info 结构体包含预期的值
	assert.Equal(t, gitVersion, info.GitVersion)
	assert.Equal(t, gitCommit, info.GitCommit)
	assert.Equal(t, gitTreeState, info.GitTreeState)
	assert.Equal(t, buildDate, info.BuildDate)
	assert.Equal(t, runtime.Version(), info.GoVersion)
	assert.Equal(t, runtime.Compiler, info.Compiler)
	assert.Equal(t, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), info.Platform)
}

func TestInfo_String(t *testing.T) {
	info := Info{GitVersion: "v1.0.0"}

	// 测试 String 方法
	assert.Equal(t, "v1.0.0", info.String())
}

func TestInfo_ToJSON(t *testing.T) {
	info := Info{
		GitVersion:   "v1.0.0",
		GitCommit:    "abc123",
		GitTreeState: "clean",
		BuildDate:    "2024-01-01T00:00:00Z",
		GoVersion:    "go1.20",
		Compiler:     "gc",
		Platform:     "linux/amd64",
	}

	// 预期的 JSON 输出
	expectedJSON := `{"gitVersion":"v1.0.0","gitCommit":"abc123","gitTreeState":"clean","buildDate":"2024-01-01T00:00:00Z","goVersion":"go1.20","compiler":"gc","platform":"linux/amd64"}`

	// 测试 ToJSON 方法
	assert.JSONEq(t, expectedJSON, info.ToJSON())
}

func TestInfo_Text(t *testing.T) {
	info := Info{
		GitVersion:   "v1.0.0",
		GitCommit:    "abc123",
		GitTreeState: "clean",
		BuildDate:    "2024-01-01T00:00:00Z",
		GoVersion:    "go1.20",
		Compiler:     "gc",
		Platform:     "linux/amd64",
	}

	// 测试 Text 方法
	stringOutput := info.Text()
	assert.Contains(t, stringOutput, "gitVersion: "+info.GitVersion)
	assert.Contains(t, stringOutput, "gitCommit: "+info.GitCommit)
	assert.Contains(t, stringOutput, "gitTreeState: "+info.GitTreeState)
	assert.Contains(t, stringOutput, "buildDate: "+info.BuildDate)
	assert.Contains(t, stringOutput, "goVersion: "+info.GoVersion)
	assert.Contains(t, stringOutput, "compiler: "+info.Compiler)
	assert.Contains(t, stringOutput, "platform: "+info.Platform)
}
