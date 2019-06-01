package lib

import "os"

//判断所给路径文件/文件夹是否存在,返回true是存在
func CheckPathExist(file string) bool {
	if _, err := os.Stat(file); err != nil { //目录不存在
		if os.IsExist(err) {
			return true
		}

		return false
	}

	return true
}
