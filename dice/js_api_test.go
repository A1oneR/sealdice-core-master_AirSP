package dice

import (
	"encoding/base64"
	"net/url"
	"runtime"
	"strings"
	"testing"
)

func TestBase64ToImageReturnsCanonicalFileURL(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows file URL regression test")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte("image-data"))
	imageURL, err := Base64ToImageFunc()(encoded)
	if err != nil {
		t.Fatalf("Base64ToImageFunc() error = %v", err)
	}
	parsed, err := url.Parse(imageURL)
	if err != nil {
		t.Fatalf("parse image URL %q: %v", imageURL, err)
	}
	if parsed.Scheme != "file" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || len(parsed.Path) < 3 || parsed.Path[2] != ':' {
		t.Fatalf("non-canonical Windows file URL: %q (host=%q path=%q)", imageURL, parsed.Host, parsed.Path)
	}
}
