package version

import (
	"fmt"
	"sync/atomic"

	utilversion "github.com/clin211/linhub/util/version"
)

var dynamicGitVersion atomic.Value

func init() {
	// 初始化为静态的 gitVersion
	dynamicGitVersion.Store(gitVersion)
}

// SetDynamicVersion 覆盖 Get() 中作为 GitVersion 返回的版本。
// 指定的版本必须非空、必须是合法的语义化版本，并且必须
// 与默认 gitVersion 的主版本/次版本/补丁版本相匹配。
func SetDynamicVersion(dynamicVersion string) error {
	if err := ValidateDynamicVersion(dynamicVersion); err != nil {
		return err
	}
	dynamicGitVersion.Store(dynamicVersion)
	return nil
}

// ValidateDynamicVersion 确保给定的版本非空、是合法的语义化版本，
// 并且与默认 gitVersion 的主版本/次版本/补丁版本相匹配。
func ValidateDynamicVersion(dynamicVersion string) error {
	return validateDynamicVersion(dynamicVersion, gitVersion)
}

func validateDynamicVersion(dynamicVersion, defaultVersion string) error {
	if len(dynamicVersion) == 0 {
		return fmt.Errorf("version must not be empty")
	}
	if dynamicVersion == defaultVersion {
		// 允许无操作（no-op）
		return nil
	}
	vRuntime, err := utilversion.ParseSemantic(dynamicVersion)
	if err != nil {
		return err
	}
	// 必须与默认版本的主版本/次版本/补丁版本相匹配
	var vDefault *utilversion.Version
	if defaultVersion == "v0.0.0-master+$Format:%H$" {
		// 特殊处理占位符值，因为它无法被解析为语义化版本
		vDefault, err = utilversion.ParseSemantic("v0.0.0-master")
	} else {
		vDefault, err = utilversion.ParseSemantic(defaultVersion)
	}
	if err != nil {
		return err
	}
	if vRuntime.Major() != vDefault.Major() || vRuntime.Minor() != vDefault.Minor() || vRuntime.Patch() != vDefault.Patch() {
		return fmt.Errorf("version %q must match major/minor/patch of default version %q", dynamicVersion, defaultVersion)
	}
	return nil
}
