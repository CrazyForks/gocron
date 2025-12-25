package i18n

import (
	"github.com/gin-gonic/gin"
)

type Locale string

const (
	ZhCN Locale = "zh-CN"
	EnUS Locale = "en-US"
)

var messages = map[Locale]map[string]string{
	ZhCN: zhCN,
	EnUS: enUS,
}

func T(c *gin.Context, key string, args ...interface{}) string {
	locale := GetLocale(c)
	msg, ok := messages[locale][key]
	if !ok {
		msg = messages[ZhCN][key]
		if msg == "" {
			return key
		}
	}
	return msg
}

// Translate 不依赖gin.Context的翻译函数，默认使用中文
func Translate(key string) string {
	msg, ok := messages[ZhCN][key]
	if !ok {
		return key
	}
	return msg
}

func GetLocale(c *gin.Context) Locale {
	lang := c.GetHeader("Accept-Language")
	if lang == "" || lang == "zh-CN" || lang == "zh" {
		return ZhCN
	}
	return EnUS
}
