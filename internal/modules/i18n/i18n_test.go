package i18n

import "testing"

func TestParseLocale(t *testing.T) {
	cases := map[string]Locale{
		"en":      EnUS,
		"en-US":   EnUS,
		"EN":      EnUS,
		"zh":      ZhCN,
		"zh-CN":   ZhCN,
		"":        ZhCN,
		"unknown": ZhCN,
	}
	for in, want := range cases {
		if got := ParseLocale(in); got != want {
			t.Errorf("ParseLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslateRespectsDefaultLocale(t *testing.T) {
	orig := defaultLocale
	defer SetDefaultLocale(orig)

	SetDefaultLocale(ZhCN)
	if got := Translate("rpc_unavailable"); got != "无法连接远程服务器" {
		t.Errorf("zh default: got %q", got)
	}

	SetDefaultLocale(EnUS)
	if got := Translate("rpc_unavailable"); got != "Unable to connect to remote server" {
		t.Errorf("en default: got %q", got)
	}
}

func TestSetDefaultLocaleIgnoresInvalid(t *testing.T) {
	orig := defaultLocale
	defer SetDefaultLocale(orig)

	SetDefaultLocale(EnUS)
	SetDefaultLocale("fr-FR") // 非法，应被忽略，保持 EnUS
	if defaultLocale != EnUS {
		t.Errorf("invalid locale should be ignored, got %q", defaultLocale)
	}
}

func TestTranslateUnknownKeyReturnsKey(t *testing.T) {
	if got := Translate("no_such_key_xyz"); got != "no_such_key_xyz" {
		t.Errorf("unknown key should return itself, got %q", got)
	}
}

func TestTranslateFallsBackToZhWhenMissingInDefault(t *testing.T) {
	orig := defaultLocale
	defer SetDefaultLocale(orig)

	// 构造一个只存在于中文包、英文包缺失的 key，验证英文默认下回退中文
	const k = "__test_only_zh_key__"
	zhCN[k] = "仅中文"
	defer delete(zhCN, k)

	SetDefaultLocale(EnUS)
	if got := Translate(k); got != "仅中文" {
		t.Errorf("expected fallback to zh, got %q", got)
	}
}
