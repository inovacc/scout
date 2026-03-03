package engine

// Surfshark API constants re-exported for test compatibility.
import "github.com/inovacc/scout/internal/engine/vpn"

const (
	surfsharkAPIBase     = vpn.SurfsharkAPIBase
	surfsharkLoginPath   = vpn.SurfsharkLoginPath
	surfsharkRenewPath   = vpn.SurfsharkRenewPath
	surfsharkUserPath    = vpn.SurfsharkUserPath
	surfsharkServersPath = vpn.SurfsharkServersPath
)

// Surfshark internal types re-exported for test compatibility.
type surfsharkLoginRequest = vpn.SurfsharkLoginRequest
