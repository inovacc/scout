package flow

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// VaultResolver adapts a vault.Handle to the SecretResolver interface so the
// runtime can resolve ${secret.NAME} at send time. The handle owns the buffers;
// the caller closes it (which zeros them) after the run.
type VaultResolver struct {
	h *vault.Handle
}

// NewVaultResolver wraps an opened vault.Handle.
func NewVaultResolver(h *vault.Handle) *VaultResolver {
	return &VaultResolver{h: h}
}

// Secret returns a COPY of the named secret's bytes. The copy is short-lived
// (consumed immediately by Interpolate into a request that is sent and discarded);
// the vault's own buffer stays locked/zeroable.
func (r *VaultResolver) Secret(name string) ([]byte, error) {
	lb, err := r.h.Secret(name)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: vault secret %q: %w", name, err)
	}
	return append([]byte(nil), lb.Bytes()...), nil
}
