package version

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version 是版本号的不透明表示。
type Version struct {
	components    []uint
	semver        bool
	preRelease    string
	buildMetadata string
}

var (
	// versionMatchRE 将版本字符串拆分为数字部分和 "extra" 部分。
	versionMatchRE = regexp.MustCompile(`^\s*v?([0-9]+(?:\.[0-9]+)*)(.*)*$`)
	// extraMatchRE 将 versionMatchRE 中的 "extra" 部分拆分为 semver 预发布信息和构建元数据；它不会校验预发布部分 "不能有前导零" 的约束。
	extraMatchRE = regexp.MustCompile(`^(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?\s*$`)
)

func parse(str string, semver bool) (*Version, error) {
	parts := versionMatchRE.FindStringSubmatch(str)
	if parts == nil {
		return nil, fmt.Errorf("could not parse %q as version", str)
	}
	numbers, extra := parts[1], parts[2]

	components := strings.Split(numbers, ".")
	if (semver && len(components) != 3) || (!semver && len(components) < 2) {
		return nil, fmt.Errorf("illegal version string %q", str)
	}

	v := &Version{
		components: make([]uint, len(components)),
		semver:     semver,
	}
	for i, comp := range components {
		if (i == 0 || semver) && strings.HasPrefix(comp, "0") && comp != "0" {
			return nil, fmt.Errorf("illegal zero-prefixed version component %q in %q", comp, str)
		}
		num, err := strconv.ParseUint(comp, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("illegal non-numeric version component %q in %q: %v", comp, str, err)
		}
		v.components[i] = uint(num)
	}

	if semver && extra != "" {
		extraParts := extraMatchRE.FindStringSubmatch(extra)
		if extraParts == nil {
			return nil, fmt.Errorf("could not parse pre-release/metadata (%s) in version %q", extra, str)
		}
		v.preRelease, v.buildMetadata = extraParts[1], extraParts[2]

		for _, comp := range strings.Split(v.preRelease, ".") {
			if _, err := strconv.ParseUint(comp, 10, 0); err == nil {
				if strings.HasPrefix(comp, "0") && comp != "0" {
					return nil, fmt.Errorf("illegal zero-prefixed version component %q in %q", comp, str)
				}
			}
		}
	}

	return v, nil
}

// HighestSupportedVersion 返回支持的最高版本。
// 此函数假定支持的最高版本必须为 v1.x。
func HighestSupportedVersion(versions []string) (*Version, error) {
	if len(versions) == 0 {
		return nil, errors.New("empty array for supported versions")
	}

	var (
		highestSupportedVersion *Version
		theErr                  error
	)

	for i := len(versions) - 1; i >= 0; i-- {
		currentHighestVer, err := ParseGeneric(versions[i])
		if err != nil {
			theErr = err
			continue
		}

		if currentHighestVer.Major() > 1 {
			continue
		}

		if highestSupportedVersion == nil || highestSupportedVersion.LessThan(currentHighestVer) {
			highestSupportedVersion = currentHighestVer
		}
	}

	if highestSupportedVersion == nil {
		return nil, fmt.Errorf(
			"could not find a highest supported version from versions (%v) reported: %+v",
			versions, theErr)
	}

	if highestSupportedVersion.Major() != 1 {
		return nil, fmt.Errorf("highest supported version reported is %v, must be v1.x", highestSupportedVersion)
	}

	return highestSupportedVersion, nil
}

// ParseGeneric 解析 "通用" 格式的版本字符串。版本字符串必须由两个或多个以点分隔的
// 数字字段组成（第一个字段不能有前导零），后面可跟任意未解释的数据（这些数据无需通过
// 标点符号与最后一个数字字段分隔）。为方便使用，会忽略前后的空白，并允许版本号以
// 字母 "v" 开头。另请参阅 ParseSemantic。
func ParseGeneric(str string) (*Version, error) {
	return parse(str, false)
}

// MustParseGeneric 类似于 ParseGeneric，区别在于解析出错时会 panic。
func MustParseGeneric(str string) *Version {
	v, err := ParseGeneric(str)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseSemantic 解析严格遵循 "语义化版本" 规范（http://semver.org/）的版本字符串
// （不过它会忽略前后空白，并允许版本号以 "v" 开头）。对于不一定遵循语义化版本
// 语法的版本字符串，请使用 ParseGeneric。
func ParseSemantic(str string) (*Version, error) {
	return parse(str, true)
}

// MustParseSemantic 类似于 ParseSemantic，区别在于解析出错时会 panic。
func MustParseSemantic(str string) *Version {
	v, err := ParseSemantic(str)
	if err != nil {
		panic(err)
	}
	return v
}

// MajorMinor 返回包含给定主版本号和次版本号的 Version。
func MajorMinor(major, minor uint) *Version {
	return &Version{components: []uint{major, minor}}
}

// Major 返回主版本号。
func (v *Version) Major() uint {
	return v.components[0]
}

// Minor 返回次版本号。
func (v *Version) Minor() uint {
	return v.components[1]
}

// Patch 在 v 是语义化版本时返回补丁版本号，否则返回 0。
func (v *Version) Patch() uint {
	if len(v.components) < 3 {
		return 0
	}
	return v.components[2]
}

// BuildMetadata 在 v 是语义化版本时返回构建元数据，否则返回空字符串。
func (v *Version) BuildMetadata() string {
	return v.buildMetadata
}

// PreRelease 在 v 是语义化版本时返回预发布元数据，否则返回空字符串。
func (v *Version) PreRelease() string {
	return v.preRelease
}

// Components 返回各版本号分量。
func (v *Version) Components() []uint {
	return v.components
}

// WithMajor 返回版本对象的副本，并使用指定的主版本号。
func (v *Version) WithMajor(major uint) *Version {
	result := *v
	result.components = []uint{major, v.Minor(), v.Patch()}
	return &result
}

// WithMinor 返回版本对象的副本，并使用指定的次版本号。
func (v *Version) WithMinor(minor uint) *Version {
	result := *v
	result.components = []uint{v.Major(), minor, v.Patch()}
	return &result
}

// WithPatch 返回版本对象的副本，并使用指定的补丁版本号。
func (v *Version) WithPatch(patch uint) *Version {
	result := *v
	result.components = []uint{v.Major(), v.Minor(), patch}
	return &result
}

// WithPreRelease 返回版本对象的副本，并使用指定的预发布信息。
func (v *Version) WithPreRelease(preRelease string) *Version {
	result := *v
	result.components = []uint{v.Major(), v.Minor(), v.Patch()}
	result.preRelease = preRelease
	return &result
}

// WithBuildMetadata 返回版本对象的副本，并使用指定的构建元数据。
func (v *Version) WithBuildMetadata(buildMetadata string) *Version {
	result := *v
	result.components = []uint{v.Major(), v.Minor(), v.Patch()}
	result.buildMetadata = buildMetadata
	return &result
}

// String 将 Version 转换回字符串；注意，对于通过 ParseGeneric 解析得到的版本，
// 返回值不会包含版本号末尾未解释的部分。
func (v *Version) String() string {
	if v == nil {
		return "<nil>"
	}
	var buffer bytes.Buffer

	for i, comp := range v.components {
		if i > 0 {
			buffer.WriteString(".")
		}
		buffer.WriteString(fmt.Sprintf("%d", comp))
	}
	if v.preRelease != "" {
		buffer.WriteString("-")
		buffer.WriteString(v.preRelease)
	}
	if v.buildMetadata != "" {
		buffer.WriteString("+")
		buffer.WriteString(v.buildMetadata)
	}

	return buffer.String()
}

// compareInternal 在 v 小于 other 时返回 -1，大于时返回 1，相等时返回 0。
func (v *Version) compareInternal(other *Version) int {
	vLen := len(v.components)
	oLen := len(other.components)
	for i := 0; i < vLen && i < oLen; i++ {
		switch {
		case other.components[i] < v.components[i]:
			return 1
		case other.components[i] > v.components[i]:
			return -1
		}
	}

	// 如果公共部分相同，但一方多出的分量并非全部为零，则该方更大。
	switch {
	case oLen < vLen && !onlyZeros(v.components[oLen:]):
		return 1
	case oLen > vLen && !onlyZeros(other.components[vLen:]):
		return -1
	}

	if !v.semver || !other.semver {
		return 0
	}

	switch {
	case v.preRelease == "" && other.preRelease != "":
		return 1
	case v.preRelease != "" && other.preRelease == "":
		return -1
	case v.preRelease == other.preRelease: // 包含两者均为 "" 的情况
		return 0
	}

	vPR := strings.Split(v.preRelease, ".")
	oPR := strings.Split(other.preRelease, ".")
	for i := 0; i < len(vPR) && i < len(oPR); i++ {
		vNum, err := strconv.ParseUint(vPR[i], 10, 0)
		if err == nil {
			oNum, err := strconv.ParseUint(oPR[i], 10, 0)
			if err == nil {
				switch {
				case oNum < vNum:
					return 1
				case oNum > vNum:
					return -1
				default:
					continue
				}
			}
		}
		if oPR[i] < vPR[i] {
			return 1
		} else if oPR[i] > vPR[i] {
			return -1
		}
	}

	switch {
	case len(oPR) < len(vPR):
		return 1
	case len(oPR) > len(vPR):
		return -1
	}

	return 0
}

// onlyZeros 在数组包含任意非零元素时返回 false。
func onlyZeros(array []uint) bool {
	for _, num := range array {
		if num != 0 {
			return false
		}
	}
	return true
}

// AtLeast 检测某个版本是否至少等于给定的最低版本。如果两个 Version 都是语义化
// 版本，则使用语义化版本的比较算法；否则仅比较数字分量，缺失的分量视为 "0"
// （例如 "1.4" 等于 "1.4.0"）。
func (v *Version) AtLeast(min *Version) bool {
	return v.compareInternal(min) != -1
}

// LessThan 检测某个版本是否小于给定版本。（它与 AtLeast 完全相反，适用于
// 询问 "v 是否过旧？" 比询问 "v 是否足够新？" 更合适的场景。）
func (v *Version) LessThan(other *Version) bool {
	return v.compareInternal(other) == -1
}

// Compare 将 v 与一个版本字符串进行比较（该字符串会根据 v 是否为语义化版本
// 决定按 Semantic 还是非 Semantic 解析）。比较成功时，若 v 小于 other 返回 -1，
// 大于返回 1，相等则返回 0。
func (v *Version) Compare(other string) (int, error) {
	ov, err := parse(other, v.semver)
	if err != nil {
		return 0, err
	}
	return v.compareInternal(ov), nil
}
