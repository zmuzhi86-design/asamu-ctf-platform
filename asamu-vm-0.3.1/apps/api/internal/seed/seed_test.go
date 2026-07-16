package seed

import "testing"

func TestDefaultAssetMIMEUsesFileExtension(t *testing.T) {
	tests := map[string]string{
		"/assets/background.png": "image/png",
		"/assets/photo.jpeg":     "image/jpeg",
		"/assets/scene.webp":     "image/webp",
		"/assets/vector.svg":     "image/svg+xml",
	}
	for path, expected := range tests {
		if actual := defaultAssetMIME(path); actual != expected {
			t.Fatalf("defaultAssetMIME(%q)=%q, want %q", path, actual, expected)
		}
	}
}
