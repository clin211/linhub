package file

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
)

// FileType 使用 filetype 包来判断给定文件路径的类型。
func FileType(filePath string) (types.Type, error) {
	file, _ := os.Open(filePath)

	// 我们只需要传入文件头部，即前 261 个字节即可。
	head := make([]byte, 261)
	_, _ = file.Read(head)

	return filetype.Match(head)
}

// FileExists 在给定路径存在时返回 true。
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	return false, err
}

// DirExists 在给定路径存在且为目录时返回 true。
func DirExists(path string) (bool, error) {
	exists, _ := FileExists(path)
	fileInfo, _ := os.Stat(path)
	if !exists || !fileInfo.IsDir() {
		return false, fmt.Errorf("path either doesn't exist, or is not a directory <%s>", path)
	}
	return true, nil
}

// Touch 在给定路径不存在时创建一个空文件。
func Touch(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	return nil
}

// EnsureDir 在给定路径不存在时在该路径创建目录。
func EnsureDir(path string) error {
	exists, err := FileExists(path)
	if !exists {
		err = os.Mkdir(path, 0o755)
		return err
	}
	return err
}

// EnsureDirAll 在给定路径创建目录，并在必要时创建尚不存在的所有父目录。
func EnsureDirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

// RemoveDir 删除给定的目录（若存在）及其全部内容。
func RemoveDir(path string) error {
	return os.RemoveAll(path)
}

// EmptyDir 递归删除给定路径下的目录内容。
func EmptyDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, name := range names {
		err = os.RemoveAll(filepath.Join(path, name))
		if err != nil {
			return err
		}
	}

	return nil
}

// ListDir 以字符串切片的形式返回给定目录路径下的内容。
func ListDir(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		path = filepath.Dir(path)
		entries, _ = os.ReadDir(path)
	}

	//nolint: prealloc
	var dirPaths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPaths = append(dirPaths, filepath.Join(path, entry.Name()))
	}
	return dirPaths
}

// GetHomeDirectory 返回用户主目录的路径。在 Unix 上为 ~，在 Windows 上为 C:\Users\UserName。
func GetHomeDirectory() string {
	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}
	return currentUser.HomeDir
}

// SafeMove 以安全模式将 src 移动到 dst。
func SafeMove(src, dst string) error {
	err := os.Rename(src, dst)
	//nolint: nestif
	if err != nil {
		fmt.Printf("[fileutil] unable to rename: \"%s\" due to %s. Falling back to copying.", src, err.Error())

		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			return err
		}

		err = out.Close()
		if err != nil {
			return err
		}

		err = os.Remove(src)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsZipFileUncompressed 在指定路径下的 zip 文件使用 0 级压缩时返回 true。
func IsZipFileUncompressed(path string) (bool, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		fmt.Printf("Error reading zip file %s: %s\n", path, err)
		return false, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() { // 跳过目录，它们始终使用 store 级别的压缩
			continue
		}
		return f.Method == 0, nil // 检查第一个真正文件的压缩级别
	}
	return false, nil
}

// WriteFile 将文件写入指定路径，必要时会创建父目录。
func WriteFile(path string, file []byte) error {
	pathErr := EnsureDirAll(filepath.Dir(path))
	if pathErr != nil {
		return fmt.Errorf("cannot ensure path %s", pathErr)
	}

	err := os.WriteFile(path, file, 0o600)
	if err != nil {
		return fmt.Errorf("write error for thumbnail %s: %s ", path, err)
	}
	return nil
}

// GetIntraDir 返回一个可用于 filepath.Join 来实现目录深度的字符串，出错时返回 ""。
// 例如，给定 pattern 为 0af63ce3c99162e9df23a997f62621c5、depth 为 2、length 为 3 时，
// 返回 0af/63c 或 0af\63c（取决于操作系统），后续可这样使用：
// filepath.Join(directory, intradir, basename)。
func GetIntraDir(pattern string, depth, length int) string {
	if depth < 1 || length < 1 || (depth*length > len(pattern)) {
		return ""
	}
	intraDir := pattern[0:length] // depth 为 1，从 pattern 中取 length 个字符
	for i := 1; i < depth; i++ {  // 每增加一层深度：在 pattern 中向右移动 length 个位置，再取 length 个字符
		intraDir = filepath.Join(
			intraDir,
			pattern[length*i:length*(i+1)],
		) // 每次通过 filepath.Join 把额外的字符追加到 intraDir
	}
	return intraDir
}

// GetParent 返回给定路径的父目录。
func GetParent(path string) *string {
	isRoot := path[len(path)-1:] == "/"
	if isRoot {
		return nil
	}
	parentPath := filepath.Clean(path + "/..")
	return &parentPath
}

// ServeFileNoCache 提供指定的文件，并确保响应中包含禁止缓存的请求头。
func ServeFileNoCache(w http.ResponseWriter, r *http.Request, filepath string) {
	w.Header().Add("Cache-Control", "no-cache")

	http.ServeFile(w, r, filepath)
}

// MatchEntries 以字符串切片形式返回目录 dir 中与正则表达式 pattern 匹配的条目。
// 出错时返回空切片。MatchEntries 不会递归，仅搜索指定的 'dir' 目录本身，不会展开。
func MatchEntries(dir, pattern string) ([]string, error) {
	var res []string
	var err error

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	files, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if re.Match([]byte(file)) {
			res = append(res, filepath.Join(dir, file))
		}
	}
	return res, err
}
