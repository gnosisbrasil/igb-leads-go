package service

import (
	"bytes"
	"encoding/base64"
	"image/color"

	"github.com/skip2/go-qrcode"
)

// Brand colors mirror the Node QRCode options.
var (
	qrDark  = color.RGBA{R: 0x23, G: 0x50, B: 0xA0, A: 0xFF}
	qrLight = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// GenerateQRPNG renders the check-in code as a 300px PNG.
func GenerateQRPNG(content string) ([]byte, error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	qr.ForegroundColor = qrDark
	qr.BackgroundColor = qrLight
	return qr.PNG(300)
}

// GenerateQRDataURL renders the PNG as a data URL.
func GenerateQRDataURL(content string) (string, error) {
	var png []byte
	png, err := GenerateQRPNG(content)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	b.WriteString("data:image/png;base64,")
	enc := base64.NewEncoder(base64.StdEncoding, &b)
	if _, err := enc.Write(png); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return b.String(), nil
}
