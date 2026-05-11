package diffcase

import (
	"encoding/base64"
	"fmt"
)

func F(strList []string, limit int, dataList *[][]byte) error {
	for j := 0; j < len(strList); j++ {
		if len(*dataList) >= limit {
			break
		}
		raw, e := base64.StdEncoding.DecodeString(strList[j])
		if e != nil {
			return fmt.Errorf("cannnot decode base64 result in redis, %w", e)
		}
		*dataList = append(*dataList, raw)
	}
	return nil
}

func PipeGetResult2(strList []string, limit int, dataList *[][]byte) error {
	for j := 0; j < len(strList); j++ {
		if len(*dataList) >= limit {
			break
		}
		raw, e := base64.StdEncoding.DecodeString(strList[j])
		if e != nil {
			return fmt.Errorf("cannot decode base64 result in redis, %w", e)
		}
		*dataList = append(*dataList, raw)
	}
	return nil
}
