// Package imagingtest — тестовые изображения модуля post: JPEG с ориентацией EXIF,
// PNG с прозрачностью, анимированные PNG и WebP (07 §9). Только для тестов.
package imagingtest

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// WebP-файлы сделаны ffmpeg (libwebp): кодировщика WebP в Go нет.
var (
	//go:embed testdata/opaque.webp
	opaqueWebP []byte
	//go:embed testdata/alpha.webp
	alphaWebP []byte
	//go:embed testdata/animated.webp
	animatedWebP []byte
)

// OpaqueWebP — WebP 400×300 без прозрачности.
func OpaqueWebP() []byte { return bytes.Clone(opaqueWebP) }

// AlphaWebP — WebP 300×200 с прозрачностью.
func AlphaWebP() []byte { return bytes.Clone(alphaWebP) }

// AnimatedWebP — анимированный WebP 200×200 из 3 кадров.
func AnimatedWebP() []byte { return bytes.Clone(animatedWebP) }

// Halves — изображение w×h: левая половина красная, правая синяя.
func Halves(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.NRGBA{R: 220, A: 255}
			if x >= w/2 {
				c = color.NRGBA{B: 220, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// JPEG — изображение Halves(w, h) в JPEG; orientation > 0 — с EXIF-ориентацией.
func JPEG(w, h, orientation int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, Halves(w, h), &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
	data := buf.Bytes()
	if orientation == 0 {
		return data
	}
	return append(append([]byte{0xFF, 0xD8}, exifSegment(orientation)...), data[2:]...)
}

// exifSegment — APP1 с одной записью IFD0: Orientation (0x0112).
func exifSegment(orientation int) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2A")
	_ = binary.Write(&tiff, binary.BigEndian, uint32(8))
	_ = binary.Write(&tiff, binary.BigEndian, uint16(1))
	_ = binary.Write(&tiff, binary.BigEndian, []uint16{0x0112, 3})
	_ = binary.Write(&tiff, binary.BigEndian, uint32(1))
	_ = binary.Write(&tiff, binary.BigEndian, []uint16{uint16(orientation), 0})
	_ = binary.Write(&tiff, binary.BigEndian, uint32(0))
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// TransparentPNG — PNG w×h: верх полупрозрачный красный, низ полностью прозрачный.
func TransparentPNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h / 2 {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 128})
		}
	}
	return encodePNG(img)
}

// OpaquePNG — непрозрачный PNG Halves(w, h).
func OpaquePNG(w, h int) []byte { return encodePNG(Halves(w, h)) }

// AnimatedPNG — PNG с чанком acTL после IHDR (APNG).
func AnimatedPNG(w, h int) []byte {
	data := OpaquePNG(w, h)
	const ihdrEnd = 8 + 4 + 4 + 13 + 4 // подпись + длина, тип, данные, CRC IHDR
	body := make([]byte, 8)
	binary.BigEndian.PutUint32(body[0:], 2) // кадров
	binary.BigEndian.PutUint32(body[4:], 0) // повторов: бесконечно
	chunk := make([]byte, 4, 4+4+len(body)+4)
	binary.BigEndian.PutUint32(chunk, uint32(len(body)))
	chunk = append(chunk, "acTL"...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	out := append([]byte{}, data[:ihdrEnd]...)
	out = append(out, chunk...)
	return append(out, data[ihdrEnd:]...)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
