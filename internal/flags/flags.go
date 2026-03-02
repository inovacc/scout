package flags

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	appName = "scout"
	prefix  = "SCOUT_"
)

var (
	appDir          string
	initOnce        sync.Once
	errInit         error
	exportMu        sync.Once
	cacheMu         sync.RWMutex
	flagCache       map[string]bool
	ignoredCommands = map[string]bool{
		"logger": true, // logger command should not create log files
	}
)

func ensureInit() error {
	initOnce.Do(func() {
		dataDir, err := os.UserCacheDir()
		if err != nil {
			errInit = fmt.Errorf("failed to get user cache dir: %w", err)
			return
		}

		appDir = filepath.Join(dataDir, appName)

		if err := os.MkdirAll(appDir, 0755); err != nil {
			errInit = fmt.Errorf("failed to create app dir: %w", err)
		}
	})

	return errInit
}

func LoadFeatureFlags() (map[string]bool, error) {
	if err := ensureInit(); err != nil {
		return nil, err
	}

	flags := make(map[string]bool)

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read feature flags dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, prefix) {
			continue
		}

		switch {
		case strings.HasSuffix(name, "_ENABLED"):
			feature := strings.TrimSuffix(strings.TrimPrefix(name, prefix), "_ENABLED")
			flags[feature] = true

		case strings.HasSuffix(name, "_DISABLED"):
			feature := strings.TrimSuffix(strings.TrimPrefix(name, prefix), "_DISABLED")
			if _, exists := flags[feature]; !exists {
				flags[feature] = false
			}
		}
	}

	// Update cache
	cacheMu.Lock()

	flagCache = flags

	cacheMu.Unlock()

	return flags, nil
}

// GetCachedFlags returns cached flags without reading from the disk.
// Returns nil if the cache is not populated. Call LoadFeatureFlags first.
func GetCachedFlags() map[string]bool {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	if flagCache == nil {
		return nil
	}

	// Return a copy to prevent mutation
	result := make(map[string]bool, len(flagCache))
	maps.Copy(result, flagCache)

	return result
}

func ExportFlagsToEnv() error {
	var err error

	exportMu.Do(func() {
		err = exportFlagsToEnv()
	})

	return err
}

func exportFlagsToEnv() error {
	flags, err := LoadFeatureFlags()
	if err != nil {
		return err
	}

	var errs []string

	for feature, enabled := range flags {
		envKey := fmt.Sprintf("%s%s", prefix, strings.ToUpper(feature))

		value := "0"
		if enabled {
			value = "1"
		}

		if err := os.Setenv(envKey, value); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", envKey, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to set env vars: %s", strings.Join(errs, "; "))
	}

	return nil
}

// getFlags returns cached flags or loads from disk if cache is empty.
func getFlags() map[string]bool {
	if cached := GetCachedFlags(); cached != nil {
		return cached
	}

	flags, err := LoadFeatureFlags()
	if err != nil {
		return nil
	}

	return flags
}

func IsFeatureSet(feature string) bool {
	flags := getFlags()
	if flags == nil {
		return false
	}

	_, exists := flags[strings.ToUpper(feature)]

	return exists
}

func IsFeatureDisabled(feature string) bool {
	flags := getFlags()
	if flags == nil {
		return false
	}

	enabled, exists := flags[strings.ToUpper(feature)]

	return exists && !enabled
}

func IsFeatureEnabled(feature string) bool {
	flags := getFlags()
	if flags == nil {
		return false
	}

	return flags[strings.ToUpper(feature)]
}

func invalidateCache() {
	cacheMu.Lock()
	flagCache = nil
	cacheMu.Unlock()
}

func EnableFeature(feature string, data string) error {
	if err := ensureInit(); err != nil {
		return err
	}

	defer invalidateCache()

	feature = strings.ToUpper(feature)

	disabled := filepath.Join(appDir, fixDisableName(feature))
	enabled := filepath.Join(appDir, fixEnableName(feature))

	// Remove disabled file if it exists
	_ = os.Remove(disabled)

	// Always create/overwrite enabled file with current data
	f, err := os.Create(enabled)
	if err != nil {
		return err
	}

	if data != "" {
		if _, err = f.WriteString(data); err != nil {
			_ = f.Close()
			return err
		}
	}

	return f.Close()
}

func DisableFeature(feature string) error {
	if err := ensureInit(); err != nil {
		return err
	}

	defer invalidateCache()

	feature = strings.ToUpper(feature)

	enabled := filepath.Join(appDir, fixEnableName(feature))
	disabled := filepath.Join(appDir, fixDisableName(feature))

	if _, err := os.Stat(enabled); os.IsNotExist(err) {
		f, err := os.Create(disabled)
		if err != nil {
			return err
		}

		return f.Close()
	}

	return os.Rename(enabled, disabled)
}

func GetFeatureData(feature string) string {
	if err := ensureInit(); err != nil {
		return ""
	}

	enabled := filepath.Join(appDir, fixEnableName(strings.ToUpper(feature)))

	if _, err := os.Stat(enabled); os.IsNotExist(err) {
		return ""
	}

	out, err := os.ReadFile(enabled)
	if err != nil {
		return ""
	}

	return strings.TrimSuffix(strings.TrimPrefix(string(out), "\n"), "\n")
}

func fixEnableName(name string) string {
	return fmt.Sprintf("%s%s_ENABLED", prefix, strings.ToUpper(name))
}

func fixDisableName(name string) string {
	return fmt.Sprintf("%s%s_DISABLED", prefix, strings.ToUpper(name))
}

// IgnoreCommand adds a command to the ignore list for logging.
// Commands in this list will not trigger log file creation.
func IgnoreCommand(command string) error {
	ignoredCommands[strings.ToLower(command)] = true
	return nil
}

// ShouldIgnoreCommand returns true if the command should skip logging.
func ShouldIgnoreCommand(command string) bool {
	return ignoredCommands[strings.ToLower(command)]
}
