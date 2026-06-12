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

// defaultLocale 是无请求上下文时（调度器 / RPC 等后台场景）使用的默认语言，
// 启动时由配置经 SetDefaultLocale 设置；默认英文，与前端默认语言保持一致。
var defaultLocale = EnUS

// SetDefaultLocale 设置服务端默认语言（用于无 gin.Context 的翻译）。
func SetDefaultLocale(l Locale) {
	if l == ZhCN || l == EnUS {
		defaultLocale = l
	}
}

// ParseLocale 把配置中的语言字符串（zh / zh-CN / en / en-US）解析为 Locale，无法识别时回退中文。
func ParseLocale(lang string) Locale {
	switch lang {
	case "en", "en-US", "en_US", "EN":
		return EnUS
	default:
		return ZhCN
	}
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

// Translate 不依赖 gin.Context 的翻译函数，使用服务端默认语言 defaultLocale。
// 默认语言缺该 key 时回退中文（最完整的语言包），仍缺则返回 key 本身。
func Translate(key string) string {
	if msg, ok := messages[defaultLocale][key]; ok {
		return msg
	}
	if msg, ok := messages[ZhCN][key]; ok {
		return msg
	}
	return key
}

func GetLocale(c *gin.Context) Locale {
	lang := c.GetHeader("Accept-Language")
	if lang == "" || lang == "zh-CN" || lang == "zh" {
		return ZhCN
	}
	return EnUS
}
