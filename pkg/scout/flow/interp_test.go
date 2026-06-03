package flow

import (
	"errors"
	"testing"
)

type fakeSecrets struct{ m map[string]string }

func (f fakeSecrets) Secret(name string) ([]byte, error) {
	v, ok := f.m[name]
	if !ok {
		return nil, errors.New("no such secret")
	}
	return []byte(v), nil
}

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"token": "tok-1", "baseURL": "https://x"}
	sec := fakeSecrets{m: map[string]string{"PASSWORD": "hunter2"}}

	got, err := Interpolate("${baseURL}/u?t=${var.token}&p=${secret.PASSWORD}", vars, sec)
	if err != nil {
		t.Fatalf("Interpolate: %v", err)
	}
	if got != "https://x/u?t=tok-1&p=hunter2" {
		t.Fatalf("got %q", got)
	}
}

func TestInterpolateUnknownVarErrors(t *testing.T) {
	if _, err := Interpolate("${nope}", map[string]string{}, nil); err == nil {
		t.Fatal("expected error for unknown var")
	}
}

func TestInterpolateSecretWithoutResolverErrors(t *testing.T) {
	if _, err := Interpolate("${secret.X}", map[string]string{}, nil); err == nil {
		t.Fatal("expected error: secret ref but no resolver")
	}
}
