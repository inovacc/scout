package engine

import (
	"github.com/inovacc/scout/internal/engine/vpn"
)

// VPNProvider re-exports vpn.Provider from sub-package.
type VPNProvider = vpn.Provider
type VPNServer = vpn.Server
type VPNConnection = vpn.Connection
type VPNStatus = vpn.Status
type VPNRotationConfig = vpn.RotationConfig
type DirectProxyOption = vpn.DirectProxyOption
type DirectProxy = vpn.DirectProxy
type SurfsharkProvider = vpn.SurfsharkProvider

// Re-exported functions.
var (
	NewDirectProxy        = vpn.NewDirectProxy
	WithDirectProxyScheme = vpn.WithDirectProxyScheme
	WithDirectProxyAuth   = vpn.WithDirectProxyAuth
	NewSurfsharkProvider  = vpn.NewSurfsharkProvider
	FilterServersByCountry = vpn.FilterServersByCountry
	ParseSurfsharkClusters = vpn.ParseSurfsharkClusters
)
