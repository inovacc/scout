package scout

// Curated data pools for realistic browser fingerprint generation.

// userAgentsWindows contains real Chrome user agents for Windows.
var userAgentsWindows = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
}

// userAgentsMac contains real Chrome user agents for macOS.
var userAgentsMac = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// userAgentsLinux contains real Chrome user agents for Linux.
var userAgentsLinux = []string{
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// userAgentsMobile contains real Chrome mobile user agents.
var userAgentsMobile = []string{
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; SM-A546B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/125.0.6422.80 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/124.0.6367.111 Mobile/15E148 Safari/604.1",
}

// screenResolution holds a width/height pair.
type screenResolution struct {
	Width  int
	Height int
}

var screenResolutionsWindows = []screenResolution{
	{1920, 1080},
	{2560, 1440},
	{1366, 768},
	{1536, 864},
	{1440, 900},
	{1680, 1050},
}

var screenResolutionsMac = []screenResolution{
	{2560, 1600},
	{1440, 900},
	{1680, 1050},
	{1920, 1080},
	{2560, 1440},
}

var screenResolutionsLinux = []screenResolution{
	{1920, 1080},
	{2560, 1440},
	{1366, 768},
	{3840, 2160},
}

var screenResolutionsMobile = []screenResolution{
	{412, 915},
	{390, 844},
	{360, 800},
	{414, 896},
	{393, 852},
}

// webglProfile pairs a vendor and renderer string.
type webglProfile struct {
	Vendor   string
	Renderer string
}

var webglProfilesWindows = []webglProfile{
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (Intel)", "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (Intel)", "ANGLE (Intel, Intel(R) Iris(R) Xe Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 6700 XT Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon(TM) Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)"},
}

var webglProfilesMac = []webglProfile{
	{"Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)"},
	{"Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M2, Unspecified Version)"},
	{"Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M3, Unspecified Version)"},
	{"Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Pro, Unspecified Version)"},
	{"Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M2 Pro, Unspecified Version)"},
}

var webglProfilesLinux = []webglProfile{
	{"Google Inc. (Intel)", "ANGLE (Intel, Mesa Intel(R) UHD Graphics 630 (CFL GT2), OpenGL 4.6)"},
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060/PCIe/SSE2, OpenGL 4.6.0)"},
	{"Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 6700 XT, OpenGL 4.6)"},
	{"Google Inc. (Intel)", "ANGLE (Intel, Mesa Intel(R) Iris(R) Xe Graphics (TGL GT2), OpenGL 4.6)"},
}

var webglProfilesMobile = []webglProfile{
	{"Qualcomm", "Adreno (TM) 740"},
	{"Qualcomm", "Adreno (TM) 730"},
	{"ARM", "Mali-G715 Immortalis MC11"},
	{"Apple GPU", "Apple GPU"},
}

// timezoneLocale pairs a timezone with a matching locale.
type timezoneLocale struct {
	Timezone string
	Locale   string
	Langs    []string
}

var timezoneLocales = []timezoneLocale{
	{"America/New_York", "en-US", []string{"en-US", "en"}},
	{"America/Chicago", "en-US", []string{"en-US", "en"}},
	{"America/Denver", "en-US", []string{"en-US", "en"}},
	{"America/Los_Angeles", "en-US", []string{"en-US", "en"}},
	{"America/Sao_Paulo", "pt-BR", []string{"pt-BR", "pt", "en"}},
	{"Europe/London", "en-GB", []string{"en-GB", "en"}},
	{"Europe/Paris", "fr-FR", []string{"fr-FR", "fr", "en"}},
	{"Europe/Berlin", "de-DE", []string{"de-DE", "de", "en"}},
	{"Europe/Madrid", "es-ES", []string{"es-ES", "es", "en"}},
	{"Europe/Rome", "it-IT", []string{"it-IT", "it", "en"}},
	{"Asia/Tokyo", "ja-JP", []string{"ja-JP", "ja", "en"}},
	{"Asia/Shanghai", "zh-CN", []string{"zh-CN", "zh", "en"}},
	{"Asia/Seoul", "ko-KR", []string{"ko-KR", "ko", "en"}},
	{"Asia/Kolkata", "hi-IN", []string{"hi-IN", "hi", "en"}},
	{"Australia/Sydney", "en-AU", []string{"en-AU", "en"}},
}

// hardwareConcurrencies is the set of realistic thread counts.
var hardwareConcurrencies = []int{4, 8, 12, 16}

// deviceMemories is the set of realistic memory amounts in GB.
var deviceMemories = []int{4, 8, 16}

// colorDepths used on desktop displays.
var colorDepths = []int{24, 30}

// pixelRatiosDesktop are common DPR values for desktop displays.
var pixelRatiosDesktop = []float64{1.0, 1.25, 1.5, 2.0}

// pixelRatiosMobile are common DPR values for mobile displays.
var pixelRatiosMobile = []float64{2.0, 2.625, 3.0, 3.5}
