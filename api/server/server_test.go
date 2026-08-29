package server

import "testing"

func TestHTTPTimeoutsDefaultToUnlimited(t *testing.T) {
	keys := []string{
		"server.readHeaderTimeout",
		"server.readTimeout",
		"server.writeTimeout",
		"server.idleTimeout",
	}
	for _, key := range keys {
		if timeout := configuredDuration(key, noHTTPTimeout); timeout != noHTTPTimeout {
			t.Errorf("configuredDuration(%q) = %s, want unlimited", key, timeout)
		}
	}
}
