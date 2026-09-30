package tui

import (
	"image/color"
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/piojosso/txt421/internal/site"
)

// Est es el estilo de un pedazo de texto. Siempre lleva fondo: si no, se vería el fondo de la
// terminal y no el del tema.
type Est struct {
	Fg, Bg          color.Color
	Negrita, Subray bool
	Crudo           bool
}

func (e Est) conFg(c color.Color) Est { e.Fg = c; return e }
func (e Est) conBg(c color.Color) Est { e.Bg = c; return e }
func (e Est) negrita() Est            { e.Negrita = true; return e }
func (e Est) subrayado() Est          { e.Subray = true; return e }

// Run: texto con un estilo.
type Run struct {
	E Est
	T string
}

// crudo arma un pedazo ya dibujado, que trae sus propios códigos de color (lo que dibuja un
// componente, como el cuadro de texto) y se pone tal cual.
func crudo(s string) Run { return Run{Est{Crudo: true}, s} }

// Linea es una fila de la pantalla hecha de pedazos con estilo.
type Linea []Run

// Ancho de la línea en columnas.
func (l Linea) Ancho() int {
	n := 0
	for _, r := range l {
		n += ancho(r.T)
	}
	return n
}

func ancho(s string) int { return ansi.StringWidth(s) }

// cache de estilos de lipgloss (armar uno por pedazo en cada cuadro es caro).
var estilos = map[Est]lipgloss.Style{}

func (e Est) lip() lipgloss.Style {
	if s, ok := estilos[e]; ok {
		return s
	}
	s := lipgloss.NewStyle()
	if e.Fg != nil {
		s = s.Foreground(e.Fg)
	}
	if e.Bg != nil {
		s = s.Background(e.Bg)
	}
	if e.Negrita {
		s = s.Bold(true)
	}
	if e.Subray {
		s = s.Underline(true)
	}
	estilos[e] = s
	return s
}

// Render dibuja la línea en exactamente w columnas, rellenando con relleno.
func (l Linea) Render(w int, relleno Est) string {
	var b strings.Builder
	n := 0
	for _, r := range l {
		if r.T == "" {
			continue
		}
		aw := ancho(r.T)
		t := r.T
		if n+aw > w {
			t = ansi.Truncate(t, w-n, "")
			aw = ancho(t)
		}
		if aw == 0 {
			continue
		}
		if r.E.Crudo {
			b.WriteString(t)
			b.WriteString("\x1b[m")
		} else {
			b.WriteString(r.E.lip().Render(t))
		}
		n += aw
		if n >= w {
			break
		}
	}
	if n < w {
		b.WriteString(relleno.lip().Render(strings.Repeat(" ", w-n)))
	}
	return b.String()
}

// txt arma una línea de un solo pedazo.
func txt(e Est, s string) Linea { return Linea{{e, s}} }

// mas concatena líneas.
func mas(ls ...Linea) Linea {
	var out Linea
	for _, l := range ls {
		out = append(out, l...)
	}
	return out
}

// espacio: n columnas de fondo.
func espacio(e Est, n int) Run {
	if n < 0 {
		n = 0
	}
	return Run{e, strings.Repeat(" ", n)}
}

// recortar corta s a w columnas con "…".
func recortar(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ancho(s) <= w {
		return s
	}
	return ansi.Truncate(s, w-1, "") + "…"
}

// completar pone l en w columnas: la recorta o le agrega fondo.
func completar(l Linea, w int, relleno Est) Linea {
	n := 0
	var out Linea
	for _, r := range l {
		aw := ancho(r.T)
		if n+aw > w {
			t := ansi.Truncate(r.T, w-n, "")
			out = append(out, Run{r.E, t})
			n += ancho(t)
			break
		}
		out = append(out, r)
		n += aw
	}
	if n < w {
		out = append(out, espacio(relleno, w-n))
	}
	return out
}

// aDerecha arma una línea con izq a la izquierda y der a la derecha, en w columnas.
func aDerecha(izq, der Linea, w int, relleno Est) Linea {
	libre := w - izq.Ancho() - der.Ancho()
	if libre < 1 {
		return completar(mas(izq, Linea{espacio(relleno, 1)}, der), w, relleno)
	}
	return mas(izq, Linea{espacio(relleno, libre)}, der)
}

// centrar pone l centrada en w columnas.
func centrar(l Linea, w int, relleno Est) Linea {
	libre := w - l.Ancho()
	if libre <= 0 {
		return completar(l, w, relleno)
	}
	return mas(Linea{espacio(relleno, libre/2)}, l, Linea{espacio(relleno, libre-libre/2)})
}

// mayus: text-transform: uppercase.
func mayus(s string) string { return strings.ToUpper(s) }

// ─── Texto de mensajes ──────────────────────────────────────────────────────────────────────

// Paleta de estilos para los tramos de un mensaje sobre un fondo dado.
type estilosTexto struct {
	normal, cita, ref, spoiler, spoilerVisto, marca Est
}

// envolver corta tramos en renglones de hasta w columnas (por palabras; las palabras más largas
// que el renglón, como los links, se parten).
func envolver(ts []site.Tramo, w int, es estilosTexto, destapar bool) []Linea {
	if w < 1 {
		w = 1
	}
	var out []Linea
	var actual Linea
	n := 0
	cerrar := func() {
		// Sin espacios colgando al final.
		for len(actual) > 0 {
			u := &actual[len(actual)-1]
			t := strings.TrimRight(u.T, " ")
			if t == u.T {
				break
			}
			n -= ancho(u.T) - ancho(t)
			if t == "" {
				actual = actual[:len(actual)-1]
				continue
			}
			u.T = t
			break
		}
		out = append(out, actual)
		actual, n = nil, 0
	}
	agregar := func(e Est, p string) {
		if len(actual) > 0 && actual[len(actual)-1].E == e {
			actual[len(actual)-1].T += p
		} else {
			actual = append(actual, Run{e, p})
		}
		n += ancho(p)
	}
	for _, t := range ts {
		if t.Estilo == site.Salto {
			cerrar()
			continue
		}
		e := es.normal
		texto := t.Texto
		switch t.Estilo {
		case site.Cita:
			e = es.cita
		case site.Ref:
			e = es.ref
		case site.Marca:
			e = es.marca
		case site.Spoiler:
			if destapar {
				e = es.spoilerVisto
			} else {
				e = es.spoiler
			}
		}
		for _, p := range palabras(texto) {
			w0 := ancho(p)
			if p == " " || strings.TrimSpace(p) == "" {
				if n == 0 {
					continue // sin espacios al principio del renglón
				}
				if n+w0 > w {
					cerrar()
					continue
				}
				agregar(e, p)
				continue
			}
			if n+w0 > w && n > 0 {
				cerrar()
			}
			// Palabra más larga que el renglón: se parte (al menos un carácter por renglón,
			// aunque sea más ancho que el renglón, para no quedar dando vueltas).
			for w0 > w-n && w0 > w {
				corte := ansi.Truncate(p, w-n, "")
				if corte == "" {
					if n > 0 {
						cerrar()
						continue
					}
					_, tam := utf8.DecodeRuneInString(p)
					corte = p[:tam]
				}
				agregar(e, corte)
				cerrar()
				p = p[len(corte):]
				w0 = ancho(p)
			}
			if p != "" {
				agregar(e, p)
			}
		}
	}
	cerrar()
	return out
}

var rePalabras = regexp.MustCompile(`\s+|\S+`)

func palabras(s string) []string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return rePalabras.FindAllString(s, -1)
}

// textoPlanoCorto: para extractos de una línea.
func textoPlanoCorto(s string, w int) string {
	s = strings.Join(strings.Fields(s), " ")
	return recortar(s, w)
}

// primeraRuna en mayúscula (para títulos).
func primeraMayus(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return strings.ToUpper(string(r)) + s[n:]
}
