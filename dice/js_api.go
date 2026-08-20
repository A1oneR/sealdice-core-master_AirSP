package dice

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"sealdice-core/logger"
)

func Base64ToImageFunc() func(string) (string, error) {
	return func(b64 string) (string, error) {
		log := logger.M()
		// 解码 Base64 值
		data, e := base64.StdEncoding.DecodeString(b64)
		if e != nil {
			// 出现错误，拒绝向下执行
			return "", errors.New("不合法的base64值：" + b64)
		}
		// 计算 MD5 哈希值作为文件名
		hash := md5.Sum(data) //nolint:gosec
		filename := hex.EncodeToString(hash[:])
		tempDir := os.TempDir()
		// 构建文件路径
		imageurlPath := filepath.Join(tempDir, filename)
		imageurlPath = filepath.ToSlash(imageurlPath)
		// 将数据写入文件
		fi, err := os.OpenFile(imageurlPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0664)
		if err != nil {
			return "", errors.New("创建文件出错:" + err.Error())
		}
		defer func(fi *os.File) {
			errClose := fi.Close()
			if errClose != nil {
				log.Errorf("关闭文件出错:%s", errClose.Error())
			}
		}(fi)
		_, err = fi.Write(data)
		if err != nil {
			return "", errors.New("写入文件出错:" + err.Error())
		}
		log.Info("File saved to:", imageurlPath)
		// url.URL 会正确生成 Windows 的 file:///C:/... 形式；直接拼接
		// "file://" + path 会得到 file://C:/...，解析时会把 C: 当成主机名。
		fileURLPath := imageurlPath
		if runtime.GOOS == "windows" && !strings.HasPrefix(fileURLPath, "/") {
			fileURLPath = "/" + fileURLPath
		}
		return (&url.URL{Scheme: "file", Path: fileURLPath}).String(), nil
	}
}
