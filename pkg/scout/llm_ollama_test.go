package scout

import "testing"

func TestNewOllamaProviderOptions(t *testing.T) {
	o := defaultOllamaOptions()

	WithOllamaHost("http://remote:11434")(o)
	if o.host != "http://remote:11434" {
		t.Errorf("host = %q, want %q", o.host, "http://remote:11434")
	}

	WithOllamaModel("mistral")(o)
	if o.model != "mistral" {
		t.Errorf("model = %q, want %q", o.model, "mistral")
	}

	WithOllamaAutoPull()(o)
	if !o.autoPull {
		t.Error("autoPull should be true")
	}
}

func TestOllamaProviderName(t *testing.T) {
	p := &OllamaProvider{model: "test"}
	if p.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", p.Name(), "ollama")
	}
}

func TestOllamaDefaultModel(t *testing.T) {
	o := defaultOllamaOptions()
	if o.model != "llama3.2" {
		t.Errorf("default model = %q, want %q", o.model, "llama3.2")
	}
}
