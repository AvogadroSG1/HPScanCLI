package discovery

import "testing"

func TestLocalIP(t *testing.T) {
	ip, err := localIP()
	if err != nil {
		t.Fatalf("localIP returned unexpected error: %v", err)
	}
	if ip == "" {
		t.Fatal("localIP returned an empty string")
	}
	t.Logf("local IP: %s", ip)
}
