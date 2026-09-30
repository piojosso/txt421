package site

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	reRef     = regexp.MustCompile(`>>(\d{1,10})`)
	reSpoiler = regexp.MustCompile(`\[spoiler\]([\s\S]*?)\[/spoiler\]`)
	reCita    = regexp.MustCompile(`^>(?:[^>]|>\D|>$|$)`)
)

// Formatear aplica al texto las mismas tres marcas que el sitio (src/format.js): >cita por línea,
// >>123 y [spoiler]…[/spoiler] dentro de una línea. Sirve para la vista previa.
func Formatear(texto string) []Tramo {
	texto = strings.ReplaceAll(strings.ReplaceAll(texto, "\r\n", "\n"), "\r", "\n")
	var out []Tramo
	for i, linea := range strings.Split(texto, "\n") {
		if i > 0 {
			out = append(out, Tramo{Estilo: Salto})
		}
		base := Normal
		if reCita.MatchString(linea) {
			base = Cita
		}
		out = append(out, formatearLinea(linea, base)...)
	}
	return out
}

func formatearLinea(linea string, base Estilo) []Tramo {
	var out []Tramo
	// Primero los spoilers, después las referencias dentro de cada pedazo.
	resto := linea
	for {
		loc := reSpoiler.FindStringSubmatchIndex(resto)
		if loc == nil {
			out = append(out, refs(resto, base)...)
			break
		}
		out = append(out, refs(resto[:loc[0]], base)...)
		out = append(out, Tramo{Estilo: Spoiler, Texto: resto[loc[2]:loc[3]]})
		resto = resto[loc[1]:]
	}
	return out
}

func refs(s string, base Estilo) []Tramo {
	var out []Tramo
	for {
		loc := reRef.FindStringSubmatchIndex(s)
		if loc == nil {
			if s != "" {
				out = append(out, Tramo{Estilo: base, Texto: s})
			}
			return out
		}
		if loc[0] > 0 {
			out = append(out, Tramo{Estilo: base, Texto: s[:loc[0]]})
		}
		n, _ := strconv.Atoi(s[loc[2]:loc[3]])
		out = append(out, Tramo{Estilo: Ref, Texto: s[loc[0]:loc[1]], Num: n})
		s = s[loc[1]:]
	}
}

// LimpiarTexto hace lo mismo que el sitio antes de guardar (limpiarTexto en app.js): saca
// invisibles y espacios al final de cada línea, junta más de dos saltos seguidos y recorta.
func LimpiarTexto(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.In(r, unicode.Cc, unicode.Cf) {
			return -1
		}
		return r
	}, s)
	s = reFinDeLinea.ReplaceAllString(s, "")
	s = reSaltos.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

var (
	reFinDeLinea = regexp.MustCompile(`(?m)[ \t]+$`)
	reSaltos     = regexp.MustCompile(`\n{3,}`)
)

// Largo cuenta como JavaScript (unidades UTF-16), que es como cuentan el sitio y su contador.
func Largo(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
