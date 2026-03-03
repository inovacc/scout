package engine

import (
	"github.com/inovacc/scout/internal/engine/vpn"
)

// vpnRotator is a type alias for the vpn sub-package Rotator.
type vpnRotator = vpn.Rotator

// newVPNRotator creates a rotator from a provider and config.
var newVPNRotator = vpn.NewRotator
