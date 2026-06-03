package plugin

import (
	"strings"
	"testing"
)

// envValue looks up a key (case-insensitive, matching Windows semantics) in a
// "KEY=VALUE" slice as produced for a plugin subprocess.
func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(k, key) {
			return v, true
		}
	}

	return "", false
}

// The plugin env builder must FAIL CLOSED: only an explicit allowlist (plus
// manifest-declared opt-ins) reaches an untrusted plugin subprocess. The old
// substring denylist failed open — any secret whose name lacked a known
// fragment (AWS_ACCESS_KEY_ID, ANTHROPIC_KEY, KUBECONFIG, ...) leaked.
func TestPluginEnv_DropsSecretsFailClosed(t *testing.T) {
	// Secrets the old denylist DID catch:
	t.Setenv("SCOUT_PASSPHRASE", "vault-secret")
	t.Setenv("SCOUT_AGENT_API_KEY", "bearer-secret")
	t.Setenv("SCOUT_PAIRING_TOKEN", "pair-secret")
	// Secrets the old denylist FAILED OPEN on (no PASSPHRASE/TOKEN/SECRET/API_KEY fragment):
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA-leak")
	t.Setenv("ANTHROPIC_KEY", "sk-ant-leak")
	t.Setenv("KUBECONFIG", "/home/u/.kube/config")
	// Allowlisted, non-secret vars plugins legitimately need:
	t.Setenv("SCOUT_HOME", "/tmp/scout")
	t.Setenv("SCOUT_CDP_ENDPOINT", "ws://127.0.0.1:9222/devtools/browser/x")

	env := pluginEnv(&Manifest{Name: "p", Version: "1", Command: "./p"})

	for _, secret := range []string{
		"SCOUT_PASSPHRASE", "SCOUT_AGENT_API_KEY", "SCOUT_PAIRING_TOKEN",
		"AWS_ACCESS_KEY_ID", "ANTHROPIC_KEY", "KUBECONFIG",
	} {
		if v, ok := envValue(env, secret); ok {
			t.Errorf("secret %s leaked into plugin env (=%q); allowlist must fail closed", secret, v)
		}
	}

	if _, ok := envValue(env, "SCOUT_HOME"); !ok {
		t.Error("SCOUT_HOME (allowlisted) missing from plugin env")
	}

	if _, ok := envValue(env, "SCOUT_CDP_ENDPOINT"); !ok {
		t.Error("SCOUT_CDP_ENDPOINT (allowlisted; existing plugins depend on it) missing from plugin env")
	}
}

// PATH must always pass so the plugin can find its interpreter / shared libs.
func TestPluginEnv_PathAllowlisted(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	env := pluginEnv(&Manifest{Name: "p"})
	if _, ok := envValue(env, "PATH"); !ok {
		t.Error("PATH must be allowlisted for the plugin subprocess")
	}
}

// A plugin may opt in to additional env vars via the manifest Env field; vars
// not declared and not on the default allowlist must remain dropped.
func TestPluginEnv_ManifestEnvOptInPassthrough(t *testing.T) {
	t.Setenv("MY_PLUGIN_FLAG", "on")
	t.Setenv("OTHER_UNDECLARED", "should-not-pass")

	base := pluginEnv(&Manifest{Name: "p"})
	if _, ok := envValue(base, "MY_PLUGIN_FLAG"); ok {
		t.Error("undeclared var must not pass through by default")
	}

	m := &Manifest{Name: "p", Env: []string{"MY_PLUGIN_FLAG"}}

	got := pluginEnv(m)
	if v, ok := envValue(got, "MY_PLUGIN_FLAG"); !ok || v != "on" {
		t.Errorf("declared manifest env var MY_PLUGIN_FLAG = %q,%v; want \"on\",true", v, ok)
	}

	if _, ok := envValue(got, "OTHER_UNDECLARED"); ok {
		t.Error("non-declared var OTHER_UNDECLARED must remain dropped")
	}
}

// Manifest validation rejects malformed env entries.
func TestManifestValidate_EnvNames(t *testing.T) {
	bad := &Manifest{Name: "p", Version: "1", Command: "./p", Capabilities: []string{"mcp_tool"}, Env: []string{"GOOD", "BAD=val"}}
	if err := bad.validate(); err == nil {
		t.Error("expected validation error for env name containing '='")
	}

	empty := &Manifest{Name: "p", Version: "1", Command: "./p", Capabilities: []string{"mcp_tool"}, Env: []string{""}}
	if err := empty.validate(); err == nil {
		t.Error("expected validation error for empty env name")
	}

	good := &Manifest{Name: "p", Version: "1", Command: "./p", Capabilities: []string{"mcp_tool"}, Env: []string{"MY_VAR", "ANOTHER_VAR"}}
	if err := good.validate(); err != nil {
		t.Errorf("valid env names rejected: %v", err)
	}
}
