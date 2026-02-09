package scout

import "time"

// Option configures a Browser instance.
type Option func(*options)

type options struct {
	headless    bool
	stealth     bool
	userAgent   string
	proxy       string
	windowW     int
	windowH     int
	timeout     time.Duration
	slowMotion  time.Duration
	ignoreCerts bool
	execPath    string
	userDataDir string
	env         []string
	incognito   bool
	noSandbox   bool
}

func defaults() *options {
	return &options{
		headless: true,
		windowW:  1920,
		windowH:  1080,
		timeout:  30 * time.Second,
	}
}

// WithHeadless sets whether the browser runs in headless mode. Default: true.
func WithHeadless(v bool) Option {
	return func(o *options) { o.headless = v }
}

// WithStealth enables stealth mode to avoid bot detection.
func WithStealth() Option {
	return func(o *options) { o.stealth = true }
}

// WithUserAgent sets a custom User-Agent string.
func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
}

// WithProxy sets the proxy server URL (e.g. "socks5://127.0.0.1:1080").
func WithProxy(proxy string) Option {
	return func(o *options) { o.proxy = proxy }
}

// WithWindowSize sets the browser window dimensions. Default: 1920x1080.
func WithWindowSize(w, h int) Option {
	return func(o *options) { o.windowW = w; o.windowH = h }
}

// WithTimeout sets the default timeout for all operations. Default: 30s.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithSlowMotion adds a delay between actions for debugging.
func WithSlowMotion(d time.Duration) Option {
	return func(o *options) { o.slowMotion = d }
}

// WithIgnoreCerts disables TLS certificate verification.
func WithIgnoreCerts() Option {
	return func(o *options) { o.ignoreCerts = true }
}

// WithExecPath sets the path to the browser executable.
func WithExecPath(path string) Option {
	return func(o *options) { o.execPath = path }
}

// WithUserDataDir sets the browser user data directory for persistent sessions.
func WithUserDataDir(dir string) Option {
	return func(o *options) { o.userDataDir = dir }
}

// WithEnv sets additional environment variables for the browser process.
func WithEnv(env ...string) Option {
	return func(o *options) { o.env = append(o.env, env...) }
}

// WithIncognito opens the browser in incognito mode.
func WithIncognito() Option {
	return func(o *options) { o.incognito = true }
}

// WithNoSandbox disables the browser sandbox. Use only in containers.
func WithNoSandbox() Option {
	return func(o *options) { o.noSandbox = true }
}
