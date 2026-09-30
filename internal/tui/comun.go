package tui

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// miles: 8000 → "8.000" (como el contador del sitio, en es-AR).
func miles(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			if Actual() == "en" {
				b.WriteByte(',')
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

var reRefHilo = regexp.MustCompile(`^(\d+)(?:#p(\d+))?$`)

// parseRefHilo: "123" o "123#p456".
func parseRefHilo(s string) (int, int) {
	m := reRefHilo.FindStringSubmatch(s)
	if m == nil {
		return atoi(s), 0
	}
	return atoi(m[1]), atoi(m[2])
}

// abrirNavegador abre una dirección en el navegador del sistema.
func abrirNavegador(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
	go cmd.Wait()
}

// ─── Piezas de interfaz ─────────────────────────────────────────────────────────────────────

// h1: "> Título" (h1::before del sitio).
func (a *App) h1(t string, w int) []Linea {
	pre := "> "
	ls := envolver([]site.Tramo{{Texto: t}}, w-len(pre), estilosTexto{normal: a.es.acento.negrita()}, false)
	for i := range ls {
		p := "  "
		if i == 0 {
			p = pre
		}
		ls[i] = mas(Linea{{a.es.tenue, p}}, ls[i])
	}
	return ls
}

// h2: título de sección, en acento.
func (a *App) h2(t string) Linea { return txt(a.es.acento.negrita(), t) }

// parrafo envuelve texto simple con un estilo.
func (a *App) parrafo(t string, w int, e Est) []Linea {
	return envolver([]site.Tramo{{Texto: t}}, w, estilosTexto{normal: e}, false)
}

// boton: "[ PUBLICAR ]" (button::before/::after del sitio). Activo = con foco, invertido.
func (a *App) boton(t string, activo bool) Linea {
	if activo {
		return txt(a.es.invertido, "[ "+mayus(t)+" ]")
	}
	return Linea{{a.es.acento.negrita(), "[ " + mayus(t) + " ]"}}
}

// secundario: botón punteado del sitio (Vista previa, Cancelar), más tenue.
func (a *App) secundario(t string, activo bool) Linea {
	if activo {
		return txt(a.es.invertido, "[ "+mayus(t)+" ]")
	}
	return Linea{{a.es.acento, "[ " + mayus(t) + " ]"}}
}

// fila de botones con sus zonas: devuelve la línea y registra un click por botón.
func (a *App) filaBotones(z *Zonas, y int, bs []Linea, acciones []func() tea.Cmd) Linea {
	var l Linea
	x := 0
	for i, b := range bs {
		if i > 0 {
			l = append(l, espacio(a.es.fondo, 1))
			x++
		}
		if i < len(acciones) && acciones[i] != nil {
			z.agregar(x, y, x+b.Ancho(), y+1, acciones[i])
		}
		l = mas(l, b)
		x += b.Ancho()
	}
	return l
}

// paginacion dibuja "← Anterior 1 [2] 3 … 12 Siguiente →" como el sitio y registra los clicks.
func (a *App) paginacion(z *Zonas, y, actual, total, w int, ir func(int) tea.Cmd) Linea {
	if total <= 1 {
		return nil
	}
	es := a.es
	var visibles []int
	if total <= 7 {
		for p := 1; p <= total; p++ {
			visibles = append(visibles, p)
		}
	} else {
		primera := actual - 1
		if primera < 2 {
			primera = 2
		}
		if primera > total-3 {
			primera = total - 3
		}
		visibles = append(visibles, 1)
		for p := primera; p <= primera+2; p++ {
			visibles = append(visibles, p)
		}
		visibles = append(visibles, total)
	}
	var l Linea
	x := 0
	pieza := func(t string, e Est, destino int) {
		if len(l) > 0 {
			l = append(l, espacio(es.fondo, 2))
			x += 2
		}
		l = append(l, Run{e, t})
		if destino > 0 {
			d := destino
			z.agregar(x, y, x+ancho(t), y+1, func() tea.Cmd { return ir(d) })
		}
		x += ancho(t)
	}
	if actual > 1 {
		pieza(T("anterior"), es.acento, actual-1)
	}
	ultima := 0
	for _, p := range visibles {
		if om := p - ultima - 1; om == 1 {
			pieza(itoa(p-1), es.acento.subrayado(), p-1)
		} else if om > 1 {
			pieza("…", es.tenue, 0)
		}
		if p == actual {
			pieza(" "+itoa(p)+" ", es.invertido, 0)
		} else {
			pieza(itoa(p), es.acento.subrayado(), p)
		}
		ultima = p
	}
	if actual < total {
		pieza(T("siguiente"), es.acento, actual+1)
	}
	if l.Ancho() > w {
		// No entra: la versión corta.
		l = nil
		x = 0
		if actual > 1 {
			pieza("←", es.acento, actual-1)
		}
		pieza(T("pagina_de", actual, total), es.tenue, 0)
		if actual < total {
			pieza("→", es.acento, actual+1)
		}
	}
	return l
}

// cargando / error para el área de contenido.
func (a *App) estadoCarga(w, alto int, err error) []Linea {
	ls := make([]Linea, alto)
	y := alto / 2
	if y >= alto {
		y = 0
	}
	if err != nil {
		for i, l := range a.parrafo(T("error")+textoError(err), w, a.es.error.negrita()) {
			if y+i < alto {
				ls[y+i] = centrar(l, w, a.es.fondo)
			}
		}
		return ls
	}
	ls[y] = centrar(txt(a.es.tenue, T("cargando")), w, a.es.fondo)
	return ls
}

// rellenar completa hasta alto líneas.
func rellenar(ls []Linea, alto int) []Linea {
	if len(ls) > alto {
		return ls[:alto]
	}
	for len(ls) < alto {
		ls = append(ls, nil)
	}
	return ls
}

// rango de índices [desde, hasta) que entra en una página.
type rango struct{ desde, hasta int }

// paginarAlturas agrupa elementos de alturas dadas en páginas de alto líneas (con sep líneas
// entre elementos). Un elemento más alto que la página va solo (y se corta al dibujar).
func paginarAlturas(alturas []int, alto, sep int) []rango {
	var out []rango
	i := 0
	for i < len(alturas) {
		usado := 0
		j := i
		for j < len(alturas) {
			h := alturas[j]
			if j > i {
				h += sep
			}
			if usado+h > alto && j > i {
				break
			}
			usado += h
			j++
		}
		out = append(out, rango{i, j})
		i = j
	}
	if len(out) == 0 {
		out = append(out, rango{0, 0})
	}
	return out
}

func paginaDe(pags []rango, i int) int {
	for p, r := range pags {
		if i >= r.desde && i < r.hasta {
			return p
		}
	}
	return max(0, len(pags)-1)
}

// caja dibuja un recuadro alrededor de contenido de ancho w (interior w-2).
type bordes struct{ si, s, sd, i, ii, id, d string }

var (
	bordeSimple   = bordes{"┌", "─", "┐", "│", "└", "┘", "│"}
	bordePunteado = bordes{"┌", "┄", "┐", "┆", "└", "┘", "┆"}
	bordeGrueso   = bordes{"┏", "━", "┓", "┃", "┗", "┛", "┃"}
)

func caja(interior []Linea, w int, b bordes, eb, relleno Est) []Linea {
	out := []Linea{txt(eb, b.si+strings.Repeat(b.s, max(0, w-2))+b.sd)}
	for _, l := range interior {
		out = append(out, mas(txt(eb, b.i), completar(l, w-2, relleno), txt(eb, b.d)))
	}
	out = append(out, txt(eb, b.ii+strings.Repeat(b.s, max(0, w-2))+b.id))
	return out
}
