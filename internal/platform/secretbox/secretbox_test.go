package secretbox

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := b.Seal([]byte("user@example.ru:123456"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "123456") {
		t.Fatal("sealed data contains plaintext")
	}
	again, _ := b.Seal([]byte("user@example.ru:123456"))
	if again == sealed {
		t.Fatal("nonce is not random")
	}
	got, err := b.Open(sealed)
	if err != nil || string(got) != "user@example.ru:123456" {
		t.Fatalf("Open = %q, %v", got, err)
	}

	other, _ := New(bytes.Repeat([]byte{2}, KeySize))
	if _, err := other.Open(sealed); !errors.Is(err, ErrOpen) {
		t.Fatalf("foreign key: %v", err)
	}
	if _, err := b.Open("not base64!"); !errors.Is(err, ErrOpen) {
		t.Fatalf("garbage: %v", err)
	}
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
}
