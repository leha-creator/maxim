package domain

import (
	"math"
	"strings"
)

// Шаблон плашки (06 §6.2, R-5). Изменение любого числа — новая PlateTemplateVersion.
const (
	// PlateHeightRatio — высота полосы от высоты изображения, не меньше PlateMinHeight.
	PlateHeightRatio = 0.08
	PlateMinHeight   = 40
	// PlateMarginRatio — поле слева и справа от ширины изображения.
	PlateMarginRatio = 0.02
	// PlateFontRatio — начальный кегль от высоты полосы.
	PlateFontRatio = 0.55
	// PlateMinFontRatio — нижний предел кегля от начального.
	PlateMinFontRatio = 0.6
	// PlateEllipsis — знак обрезки названия.
	PlateEllipsis = "…"
)

// PlateText — текст плашки (06 §6.2).
func PlateText(name, inn string) string { return plateText(name, "", inn) }

func plateText(name, suffix, inn string) string {
	return "Реклама. " + name + suffix + ", ИНН " + inn
}

// TextMeasurer — ширина строки в пикселях при кегле size (пикселях). Реализация на
// шрифте — post/imaging.
type TextMeasurer interface {
	Width(text string, size float64) float64
}

// PlateLayout — раскладка плашки на изображении.
type PlateLayout struct {
	// BarY — верхний край полосы; полоса — от BarY до низа изображения.
	BarY      int
	BarHeight int
	// Margin — поле слева и справа.
	Margin   int
	FontSize float64
	Text     string
	// Truncated — название обрезано с многоточием.
	Truncated bool
}

// LayoutPlate раскладывает плашку (06 §6.2, R-5): полоса во всю ширину внизу высотой
// 8% изображения, не меньше 40 px; кегль от 55% высоты полосы уменьшается до 60%
// от начального; если и так не помещается — название обрезается с «…», ИНН целиком.
func LayoutPlate(width, height int, name, inn string, m TextMeasurer) PlateLayout {
	bar := max(int(math.Round(float64(height)*PlateHeightRatio)), PlateMinHeight)
	bar = min(bar, height)
	margin := max(int(math.Round(float64(width)*PlateMarginRatio)), 1)
	avail := float64(width - 2*margin)
	l := PlateLayout{BarY: height - bar, BarHeight: bar, Margin: margin}

	size0 := float64(bar) * PlateFontRatio
	minSize := size0 * PlateMinFontRatio
	full := PlateText(name, inn)
	if w := m.Width(full, size0); w <= avail {
		l.FontSize, l.Text = size0, full
		return l
	} else if size := size0 * avail / w; size >= minSize {
		// Ширина растёт с кеглем линейно; запас на округление в измерителе.
		for size > minSize && m.Width(full, size) > avail {
			size -= 0.25
		}
		if m.Width(full, size) <= avail {
			l.FontSize, l.Text = size, full
			return l
		}
	}

	// Нижний кегль: подбираем самую длинную часть названия, с которой текст помещается.
	l.FontSize, l.Truncated = minSize, true
	runes := []rune(strings.TrimSpace(name))
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if m.Width(plateText(strings.TrimSpace(string(runes[:mid])), PlateEllipsis, inn), minSize) <= avail {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	l.Text = plateText(strings.TrimSpace(string(runes[:lo])), PlateEllipsis, inn)
	return l
}
