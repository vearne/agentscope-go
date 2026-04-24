package message

import "testing"

func TestNewBase64Source(t *testing.T) {
	s := NewBase64Source("image/png", "iVBORw==")
	if GetSourceType(s) != "base64" {
		t.Fatalf("expected base64, got %s", GetSourceType(s))
	}
	if GetSourceMediaType(s) != "image/png" {
		t.Fatalf("expected image/png, got %s", GetSourceMediaType(s))
	}
	if GetSourceData(s) != "iVBORw==" {
		t.Fatalf("expected iVBORw==, got %s", GetSourceData(s))
	}
}

func TestNewURLSource(t *testing.T) {
	s := NewURLSource("https://example.com/img.png")
	if GetSourceType(s) != "url" {
		t.Fatalf("expected url, got %s", GetSourceType(s))
	}
	if GetSourceURL(s) != "https://example.com/img.png" {
		t.Fatalf("expected url, got %s", GetSourceURL(s))
	}
}
