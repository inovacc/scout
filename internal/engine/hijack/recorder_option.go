package hijack

// RecorderOption configures a NetworkRecorder.
type RecorderOption func(*RecorderOptions)

// RecorderOptions holds recorder configuration.
type RecorderOptions struct {
	CaptureBody    bool
	CreatorName    string
	CreatorVersion string
}

// RecorderDefaults returns default recorder options.
func RecorderDefaults() *RecorderOptions {
	return &RecorderOptions{
		CreatorName:    "scout",
		CreatorVersion: "0.1.0",
	}
}

// WithCaptureBody enables or disables response body capture. Default: false.
func WithCaptureBody(v bool) RecorderOption {
	return func(o *RecorderOptions) { o.CaptureBody = v }
}

// WithCreatorName sets the creator name and version in the exported HAR.
func WithCreatorName(name, version string) RecorderOption {
	return func(o *RecorderOptions) {
		o.CreatorName = name
		o.CreatorVersion = version
	}
}
