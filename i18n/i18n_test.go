package i18n

import (
	"fmt"
	"testing"

	"golang.org/x/text/language"
)

// //go:embed locales
// var fs embed.FS

func TestNew(t *testing.T) {
	i := New()
	// 1. 添加目录
	i.Add("./locales")

	// 2. 添加文件
	// i.Add("./locales/en.yml")
	// i.Add("./locales/zh.yml")

	// 3. 添加嵌入式文件系统
	// i.AddFs(fs)

	fmt.Println(i.T("common.hello"))
	fmt.Println(i.Select(language.Chinese).T("common.hello"))
}
