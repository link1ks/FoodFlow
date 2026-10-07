package app

import (
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// The edge gateway overwrites X-Real-IP. Only its exact, explicitly configured
// addresses may supply it; an empty configuration trusts no forwarded headers.
func configureTrustedProxies(r *gin.Engine, value string) {
	var proxies []string
	if strings.TrimSpace(value) != "" {
		for _, item := range strings.Split(value, ",") {
			ip, err := netip.ParseAddr(strings.TrimSpace(item))
			if err != nil || ip.IsUnspecified() || ip.IsMulticast() {
				panic("TRUSTED_PROXIES must contain exact gateway IP addresses")
			}
			proxies = append(proxies, ip.Unmap().String())
		}
	}
	r.RemoteIPHeaders = []string{"X-Real-IP"}
	// Do not allow a platform header to bypass the explicit peer allowlist.
	r.TrustedPlatform = ""
	if err := r.SetTrustedProxies(proxies); err != nil {
		panic("invalid TRUSTED_PROXIES")
	}
}
