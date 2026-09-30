package tui

import (
	"fmt"
	"image/color"
	"strconv"
)

// Tema: los colores del sitio (public/style.css), con los mismos nombres de variable.
type Tema struct {
	Nombre     string
	Fondo      color.Color
	Caja       color.Color
	Fosforo    color.Color // acento: bordes, barras de título, links
	Texto      color.Color
	Tenue      color.Color
	Borde      color.Color
	Cita       color.Color
	Aviso      color.Color
	Error      color.Color
	SobreBorde color.Color
	// En descanso y monocromo la barra del OP no va llena (va con el fondo de la caja).
	BarraOpLlena bool
	BordeOp      color.Color // borde del mensaje inicial
}

func hex(s string) color.Color {
	v, _ := strconv.ParseUint(s[1:], 16, 32)
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

// Temas en el orden del sitio.
var Temas = []Tema{
	{"oscuro", hex("#020803"), hex("#041209"), hex("#33ff66"), hex("#9dffb4"), hex("#3fa35d"), hex("#1d6b35"), hex("#d7ff6b"), hex("#ffcc4d"), hex("#ff5f56"), hex("#d9ffe3"), true, hex("#33ff66")},
	{"claro", hex("#f5eddc"), hex("#fbf7ee"), hex("#7b1e2c"), hex("#2b211c"), hex("#6e5d52"), hex("#8c5a5f"), hex("#1f4f8a"), hex("#8a5a00"), hex("#b3261e"), hex("#fbf7ee"), true, hex("#7b1e2c")},
	{"descanso", hex("#191a1d"), hex("#212327"), hex("#5cc988"), hex("#d2d0c8"), hex("#8e939b"), hex("#33363c"), hex("#c2d38a"), hex("#e0c068"), hex("#e3897e"), hex("#191a1d"), false, hex("#5cc988")},
	{"monocromo", hex("#161616"), hex("#1c1c1c"), hex("#e4dfd4"), hex("#bfbab0"), hex("#8a857c"), hex("#2e2e2e"), hex("#a9b59a"), hex("#d4b56a"), hex("#de8a7e"), hex("#161616"), false, hex("#77736b")},
}

// TemaPorNombre devuelve el tema, oscuro si no existe.
func TemaPorNombre(n string) Tema {
	for _, t := range Temas {
		if t.Nombre == n {
			return t
		}
	}
	return Temas[0]
}

// mezclar hace lo de color-mix(in srgb, a p%, b).
func mezclar(a, b color.Color, p float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	m := func(x, y uint32) uint8 { return uint8((float64(x>>8)*p + float64(y>>8)*(1-p)) + 0.5) }
	return color.RGBA{m(ar, br), m(ag, bg), m(ab, bb), 0xff}
}

func clave(c color.Color) string {
	if c == nil {
		return "-"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("%02x%02x%02x", r>>8, g>>8, b>>8)
}
