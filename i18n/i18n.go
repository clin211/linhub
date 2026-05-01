package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// I18n 用于存储国际化的选项和配置。
type I18n struct {
	ops       Options
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
	lang      language.Tag
}

// New 使用给定的选项创建一个新的 I18n 结构体实例。
// 它接收一个可变参数的函数选项，并返回指向 I18n 结构体的指针。
func New(options ...func(*Options)) (rp *I18n) {
	ops := getOptionsOrSetDefault(nil)
	for _, f := range options {
		f(ops)
	}
	bundle := i18n.NewBundle(ops.language)
	localizer := i18n.NewLocalizer(bundle, ops.language.String())
	switch ops.format {
	case "toml":
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	case "json":
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	default:
		bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	}
	rp = &I18n{
		ops:       *ops,
		bundle:    bundle,
		localizer: localizer,
		lang:      ops.language,
	}
	for _, item := range ops.files {
		// 文件加载失败时不打断初始化（保留向后兼容），仅打印日志
		if err := rp.Add(item); err != nil {
			slog.Warn("i18n: failed to add file", "file", item, "error", err)
		}
	}
	rp.AddFS(ops.fs)
	return
}

// Select 可以切换语言。
func (i I18n) Select(lang language.Tag) *I18n {
	if lang.String() == "und" {
		lang = i.ops.language
	}
	return &I18n{
		ops:       i.ops,
		bundle:    i.bundle,
		localizer: i18n.NewLocalizer(i.bundle, lang.String()),
		lang:      lang,
	}
}

// Language 获取当前语言。
func (i I18n) Language() language.Tag {
	return i.lang
}

// LocalizeT 本地化给定的消息并返回本地化后的字符串。
// 如果无法翻译，则返回消息 ID 作为默认消息。
func (i I18n) LocalizeT(message *i18n.Message) (rp string) {
	if message == nil {
		return ""
	}

	var err error
	rp, err = i.localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: message,
	})
	if err != nil {
		// 无法翻译时使用 id 作为默认消息
		rp = message.ID
	}
	return
}

// LocalizeE 是 LocalizeT 方法的封装，它将本地化字符串转换为 error 类型并返回。
func (i I18n) LocalizeE(message *i18n.Message) error {
	return errors.New(i.LocalizeT(message))
}

// T 本地化具有给定 ID 的消息并返回本地化字符串。
// 它使用 LocalizeT 方法执行翻译。
func (i I18n) T(id string) (rp string) {
	return i.LocalizeT(&i18n.Message{ID: id})
}

// E 是 T 的封装，它将本地化字符串转换为 error 类型并返回。
func (i I18n) E(id string) error {
	return errors.New(i.T(id))
}

// Add 添加语言文件或目录（根据文件名自动获取语言）。
// 出现错误（路径不存在、文件加载失败等）时返回错误；调用方可选择忽略。
func (i *I18n) Add(f string) error {
	info, err := os.Stat(f)
	if err != nil {
		return fmt.Errorf("i18n: stat %q: %w", f, err)
	}
	if !info.IsDir() {
		if _, err := i.bundle.LoadMessageFile(f); err != nil {
			return fmt.Errorf("i18n: load message file %q: %w", f, err)
		}
		return nil
	}

	return filepath.Walk(f, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		if _, err := i.bundle.LoadMessageFile(path); err != nil {
			return fmt.Errorf("i18n: load message file %q: %w", path, err)
		}
		return nil
	})
}

// AddFS 添加嵌入式语言文件。
func (i *I18n) AddFS(fs embed.FS) {
	files := readFS(fs, ".")
	for _, name := range files {
		i.bundle.LoadMessageFileFS(fs, name)
	}
}

func readFS(fs embed.FS, dir string) (rp []string) {
	rp = make([]string, 0)
	dirs, err := fs.ReadDir(dir)
	if err != nil {
		return
	}
	for _, item := range dirs {
		name := dir + string(os.PathSeparator) + item.Name()
		if dir == "." {
			name = item.Name()
		}
		if item.IsDir() {
			rp = append(rp, readFS(fs, name)...)
		} else {
			rp = append(rp, name)
		}
	}
	return
}
