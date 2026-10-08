package handler

import (
	"strings"
	"testing"
)

func TestBuildVoucherHTML(t *testing.T) {
	html := buildVoucherHTML(
		"Seu voucher - Curso",
		"Apresente este QR Code na entrada do evento.",
		"https://leads.gnosisbrasil.com/checkin/ABC123",
		"https://api-leads.gnosisbrasil.com/api/qr/ABC123?size=600",
	)
	for _, want := range []string{
		`og:image" content="https://api-leads.gnosisbrasil.com/api/qr/ABC123?size=600"`,
		`og:image:width" content="600"`,
		`http-equiv="refresh" content="0;url=https://leads.gnosisbrasil.com/checkin/ABC123"`,
		`window.location.href="https://leads.gnosisbrasil.com/checkin/ABC123"`,
		"Seu voucher - Curso",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("faltando %q em:\n%s", want, html)
		}
	}
}

func TestBuildVoucherHTMLEscapesTitle(t *testing.T) {
	html := buildVoucherHTML(`Voucher <script>alert(1)</script>`, "d", "https://x/v", "https://x/qr")
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("título não escapado")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("escape ausente:\n%s", html)
	}
}
