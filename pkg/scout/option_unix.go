//go:build !windows

package scout

// WithXvfb enables Xvfb (X Virtual Framebuffer) for headful mode on systems
// without a display server. Optional args are passed to xvfb-run.
func WithXvfb(args ...string) Option {
	return func(o *options) {
		o.xvfb = true
		o.xvfbArgs = args
	}
}
