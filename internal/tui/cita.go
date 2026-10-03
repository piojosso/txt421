package tui

import (
	"context"
	"regexp"
	"time"

	tea "charm.land/bubbletea/v2"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// Citas como en public/citas.js del sitio: tocar un >>N (en el texto o en "Respuestas:") muestra
// una copia del mensaje citado flotando junto al link, sin moverse de donde uno está. Si el
// mensaje es de otra publicación, se trae. Enter va hasta el mensaje; Esc o un click afuera cierra.

var reRefLink = regexp.MustCompile(`^>>(\d+)$`)

// zonasRefs registra un click por cada >>N subrayado de las líneas (que empiezan en y0).
func zonasRefs(ls []Linea, y0 int, z *Zonas, accion func(n, y int) tea.Cmd) {
	for i, l := range ls {
		x := 0
		for _, r := range l {
			w := ancho(r.T)
			if r.E.Subray {
				if m := reRefLink.FindStringSubmatch(r.T); m != nil {
					n, y := atoi(m[1]), y0+i
					z.agregar(x, y, x+w, y+1, func() tea.Cmd { return accion(n, y) })
				}
			}
			x += w
		}
	}
}

// capaCita: la copia flotante de un mensaje citado.
type capaCita struct {
	hilo   *pantHilo // la publicación desde la que se citó
	num    int
	post   *site.Post
	asunto string // si es de otra publicación
	otro   int    // No. de esa otra publicación
	err    error
	y      int // línea (de la pantalla) del link; -1 = centrada
}

type citaMsg struct {
	c      *capaCita
	post   *site.Post
	asunto string
	otro   int
	err    error
}

// mostrarCita abre la copia del mensaje n; yLink es la línea del link en el área de contenido.
func (p *pantHilo) mostrarCita(a *App, n, yLink int) tea.Cmd {
	c := &capaCita{hilo: p, num: n, y: yLink}
	for i := range p.hilo.Posts {
		if p.hilo.Posts[i].No == n {
			post := p.hilo.Posts[i]
			c.post = &post
			a.capa = c
			return nil
		}
	}
	a.capa = c
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		h, err := a.cliente.Ubicar(ctx, n)
		if err != nil {
			return citaMsg{c: c, err: err}
		}
		hilo, err := a.cliente.Hilo(ctx, h)
		if err != nil {
			return citaMsg{c: c, err: err}
		}
		for _, post := range hilo.Posts {
			if post.No == n {
				post := post
				return citaMsg{c, &post, hilo.Asunto, h, nil}
			}
		}
		return citaMsg{c: c, err: site.ErrNoEncontrado}
	}
}

func (c *capaCita) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	if m, ok := msg.(citaMsg); ok && m.c == c {
		c.post, c.asunto, c.otro, c.err = m.post, m.asunto, m.otro, m.err
	}
	return nil
}

func (c *capaCita) Teclas() []Atajo {
	return []Atajo{{"Enter", T("cita_ir")}, {"Esc", T("cerrar")}}
}

// ir: hasta el mensaje (en esta publicación se elige y Esc vuelve; si es de otra, se abre).
func (c *capaCita) ir(a *App) tea.Cmd {
	a.capa = nil
	p := c.hilo
	for i, post := range p.hilo.Posts {
		if post.No == c.num {
			p.historial = append(p.historial, p.sel)
			p.sel, p.destapar, p.citaIdx = i, false, 0
			return nil
		}
	}
	if c.otro > 0 {
		return a.Ir(nuevaHilo(c.otro, c.num))
	}
	return a.Ir(nuevaHiloPorPost(c.num))
}

func (c *capaCita) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		return c.ir(a)
	case "i":
		// Seguir recorriendo las citas del mensaje elegido.
		a.capa = nil
		return c.hilo.irACita(a)
	default:
		a.capa = nil
	}
	return nil
}

func (c *capaCita) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	t := a.tema
	w := min(W-4, 84)
	iw := w - 4
	base := Est{Bg: t.Caja, Fg: t.Texto}
	tenue := base.conFg(t.Tenue)
	titulo := ">>" + itoa(c.num)
	if c.asunto != "" {
		titulo += " · " + c.asunto
	}
	in := []Linea{txt(base.conFg(t.Fosforo).negrita(), recortar(titulo, iw))}
	zi := &Zonas{}
	switch {
	case c.err != nil:
		in = append(in, txt(base.conFg(t.Error).negrita(), recortar(T("error")+textoError(c.err), iw)))
	case c.post == nil:
		in = append(in, txt(tenue, T("cargando")))
	default:
		post := *c.post
		post.Inicial = false // la copia va sin la barra llena, como la .flotante del sitio
		// dibujarPost la sangra como respuesta: se le saca la sangría.
		sg := sangriaRespuesta
		if iw+sg < 50 {
			sg = 1
		}
		lp := a.dibujarPost(post, iw+sg, opcionesPost{})
		for i := range lp {
			lp[i] = cortarLinea(lp[i], sg, iw+sg)
		}
		// Máximo lo que entra en la pantalla.
		maxL := max(3, H-10)
		if len(lp) > maxL {
			lp = append(lp[:maxL-1], txt(tenue, "   "+T("sigue")+" (Enter)"))
		}
		y0 := len(in) + 1 // borde de la ventana
		in = append(in, lp...)
		// Las citas de la copia también se pueden seguir.
		zonasRefs(lp, y0, zi, func(n, _ int) tea.Cmd { return c.hilo.mostrarCita(a, n, c.y) })
	}
	in = append(in, txt(tenue, "Enter "+T("cita_ir")+" · Esc "+T("cerrar")))
	ls := a.ventana(in, w)

	// Arriba del link si entra (como citas.js); si no, abajo.
	anchoCont := min(W-2, maxContenido)
	x := (W - anchoCont) / 2
	y := (H - len(ls)) / 2
	if c.y >= 0 {
		yLink := c.y + 3 // la cabecera ocupa 3 líneas
		y = yLink - len(ls)
		if y < 3 {
			y = yLink + 1
		}
		if y+len(ls) > H-2 {
			y = max(0, H-2-len(ls))
		}
	}
	// Click en la copia: ir al mensaje (salvo en una cita de adentro, que va primero).
	z.sumar(zi, 2, 0) // borde y espacio de la ventana
	z.agregar(0, 0, w, len(ls), func() tea.Cmd { return c.ir(a) })
	return ls, x, max(0, y)
}
