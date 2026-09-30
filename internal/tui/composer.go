package tui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/piojosso/txt421/internal/config"
	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// Campos del formulario, en el orden del Tab.
const (
	cTablon = iota
	cAsunto
	cMensaje
	cPublicar
	cPrevia
	cCancelar
	cSage
)

// pantComposer: el formulario de responder o de publicar (form.form-post del sitio).
type pantComposer struct {
	respuesta  bool
	hilo       *pantHilo // al responder
	hiloNo     int
	asuntoHilo string
	seccion    int // índice en site.Secciones; -1 = sin elegir (desde la portada)

	asunto textinput.Model
	area   textarea.Model
	sage   bool
	foco   int
	previa bool

	enviando   bool
	desde      time.Time
	err        string
	clave      string
	recuperado bool
}

type publicadoMsg struct {
	c   *pantComposer
	pub *site.Publicado
	err error
}
type tickEnvio struct{ c *pantComposer }

func (a *App) nuevoCampos(c *pantComposer) {
	t := a.tema
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = site.MaxAsunto * 2
	ti.SetVirtualCursor(true)
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = site.MaxCuerpo * 2
	ta.MaxHeight = 0
	ta.EndOfBufferCharacter = ' '
	ta.SetVirtualCursor(true)
	c.asunto, c.area = ti, ta
	c.aplicarEstilos(t)
}

func (c *pantComposer) aplicarEstilos(t Tema) {
	base := lipgloss.NewStyle().Background(t.Fondo).Foreground(t.Texto)
	tenue := base.Foreground(t.Tenue)
	st := c.area.Styles()
	for _, s := range []*textarea.StyleState{&st.Focused, &st.Blurred} {
		s.Base = base
		s.Text = base
		s.CursorLine = base
		s.Placeholder = tenue
		s.EndOfBuffer = base
		s.Prompt = base
		s.LineNumber = tenue
		s.CursorLineNumber = tenue
	}
	st.Cursor.Color = t.Fosforo
	c.area.SetStyles(st)
	si := c.asunto.Styles()
	for _, s := range []*textinput.StyleState{&si.Focused, &si.Blurred} {
		s.Text = base
		s.Placeholder = tenue
		s.Prompt = base
		s.Suggestion = tenue
	}
	si.Cursor.Color = t.Fosforo
	c.asunto.SetStyles(si)
}

// nuevaComposerRespuesta: responder en una publicación; cita > 0 agrega ">>N" (el botón
// "Responder" de cada mensaje en el sitio).
func nuevaComposerRespuesta(a *App, h *pantHilo, cita int) *pantComposer {
	c := &pantComposer{respuesta: true, hilo: h, hiloNo: h.hilo.No, asuntoHilo: h.hilo.Asunto, seccion: -1}
	a.nuevoCampos(c)
	c.clave = "respuesta:" + itoa(c.hiloNo)
	if b, ok := a.borradores[c.clave]; ok {
		c.area.SetValue(b.Cuerpo)
		c.sage = b.Sage
		c.recuperado = b.Cuerpo != ""
	}
	if cita > 0 {
		// Como formularios.js: >>N en una línea propia, al final, salvo que ya esté.
		v := c.area.Value()
		ref := ">>" + itoa(cita)
		if !regexp.MustCompile(regexp.QuoteMeta(ref) + `(\D|$)`).MatchString(v) {
			if v != "" && !strings.HasSuffix(v, "\n") {
				v += "\n"
			}
			c.area.SetValue(v + ref + "\n")
		}
	}
	c.area.MoveToEnd()
	c.enfocar(cMensaje)
	return c
}

// nuevaComposerHilo: publicación nueva; seccion "" = elegir el tablón (como en la portada).
func nuevaComposerHilo(a *App, seccion string) *pantComposer {
	c := &pantComposer{seccion: -1}
	for i, s := range site.Secciones {
		if s.Slug == seccion {
			c.seccion = i
		}
	}
	a.nuevoCampos(c)
	c.clave = "nuevo"
	if b, ok := a.borradores[c.clave]; ok {
		c.asunto.SetValue(b.Asunto)
		c.area.SetValue(b.Cuerpo)
		if c.seccion < 0 {
			for i, s := range site.Secciones {
				if s.Slug == b.Seccion {
					c.seccion = i
				}
			}
		}
		c.recuperado = b.Cuerpo != "" || b.Asunto != ""
	}
	switch {
	case c.seccion < 0:
		c.enfocar(cTablon)
	case c.asunto.Value() == "":
		c.enfocar(cAsunto)
	default:
		c.enfocar(cMensaje)
	}
	return c
}

func (c *pantComposer) Ocupada() bool { return c.enviando }

func (c *pantComposer) Titulo() string {
	if c.respuesta {
		return T("responder")
	}
	return T("publicar")
}

func (c *pantComposer) SeccionActiva() string {
	if c.seccion >= 0 {
		return site.Secciones[c.seccion].Slug
	}
	if c.hilo != nil {
		return c.hilo.SeccionActiva()
	}
	return ""
}

func (c *pantComposer) Recargar(a *App) tea.Cmd {
	if c.recuperado {
		a.avisar(T("borrador"))
		c.recuperado = false
	}
	return nil
}

func (c *pantComposer) campos() []int {
	if c.respuesta {
		return []int{cMensaje, cPublicar, cPrevia, cCancelar, cSage}
	}
	return []int{cTablon, cAsunto, cMensaje, cPublicar, cPrevia, cCancelar}
}

func (c *pantComposer) enfocar(f int) {
	c.foco = f
	c.asunto.Blur()
	c.area.Blur()
	switch f {
	case cAsunto:
		c.asunto.Focus()
	case cMensaje:
		if !c.previa {
			c.area.Focus()
		}
	}
}

func (c *pantComposer) siguiente(d int) {
	cs := c.campos()
	i := 0
	for j, f := range cs {
		if f == c.foco {
			i = j
		}
	}
	i = (i + d + len(cs)) % len(cs)
	c.enfocar(cs[i])
}

// guardarBorrador actualiza el borrador en memoria (se escribe al salir de la pantalla).
func (c *pantComposer) guardarBorrador(a *App) {
	b := config.Borrador{Cuerpo: c.area.Value(), Sage: c.sage, T: time.Now()}
	if !c.respuesta {
		b.Asunto = c.asunto.Value()
		if c.seccion >= 0 {
			b.Seccion = site.Secciones[c.seccion].Slug
		}
	}
	if strings.TrimSpace(b.Cuerpo) == "" && strings.TrimSpace(b.Asunto) == "" {
		delete(a.borradores, c.clave)
		return
	}
	a.borradores[c.clave] = b
}

// ─── Teclas ─────────────────────────────────────────────────────────────────────────────────

func (c *pantComposer) Teclas(a *App) []Atajo {
	at := []Atajo{{"Tab", T("k_campo")}, {"Ctrl+Enter/Ctrl+S", T("k_publicar")}, {"Ctrl+P", T("k_previa")}}
	if c.foco == cTablon {
		at = append([]Atajo{{"←→", T("k_elegir")}}, at...)
	}
	return append(at, Atajo{"Esc", T("k_volver")})
}

func (c *pantComposer) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	if c.enviando {
		return true, nil // mientras revisa, nada (el sitio desactiva el botón)
	}
	s := k.String()
	switch s {
	case "ctrl+enter", "ctrl+s", "alt+enter", "f2":
		return true, c.publicar(a)
	case "ctrl+p", "f3":
		c.alternarPrevia()
		return true, nil
	case "tab":
		c.siguiente(1)
		return true, nil
	case "shift+tab":
		c.siguiente(-1)
		return true, nil
	case "esc":
		c.guardarBorrador(a)
		a.guardarBorradores()
		return true, a.Volver()
	}
	switch c.foco {
	case cTablon:
		switch s {
		case "left", "up":
			if c.seccion <= 0 {
				c.seccion = len(site.Secciones) - 1
			} else {
				c.seccion--
			}
		case "right", "down", "space":
			c.seccion = (c.seccion + 1) % len(site.Secciones)
		case "enter":
			if c.seccion < 0 {
				c.seccion = 0
			}
			c.siguiente(1)
		default:
			// 1..5 elige directo.
			if len(s) == 1 && s[0] >= '1' && s[0] <= '5' {
				c.seccion = int(s[0] - '1')
			}
		}
		c.guardarBorrador(a)
		return true, nil
	case cAsunto:
		if s == "enter" || s == "down" {
			c.siguiente(1)
			return true, nil
		}
		var cmd tea.Cmd
		c.asunto, cmd = c.asunto.Update(k)
		c.guardarBorrador(a)
		return true, cmd
	case cMensaje:
		if c.previa {
			if s == "enter" || s == "e" {
				c.alternarPrevia()
			}
			return true, nil
		}
		var cmd tea.Cmd
		c.area, cmd = c.area.Update(k)
		c.guardarBorrador(a)
		return true, cmd
	case cPublicar:
		if s == "enter" || s == "space" {
			return true, c.publicar(a)
		}
	case cPrevia:
		if s == "enter" || s == "space" {
			c.alternarPrevia()
		}
	case cCancelar:
		if s == "enter" || s == "space" {
			return true, c.cancelar(a)
		}
	case cSage:
		if s == "enter" || s == "space" {
			c.sage = !c.sage
			c.guardarBorrador(a)
		}
	}
	switch s {
	case "left", "up":
		c.siguiente(-1)
	case "right", "down":
		c.siguiente(1)
	}
	return true, nil
}

func (c *pantComposer) alternarPrevia() {
	c.previa = !c.previa
	c.enfocar(cMensaje)
}

// cancelar: como el "Cancelar" del sitio, vacía el formulario y borra el borrador.
func (c *pantComposer) cancelar(a *App) tea.Cmd {
	delete(a.borradores, c.clave)
	a.guardarBorradores()
	return a.Volver()
}

func (c *pantComposer) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.PasteMsg:
		var cmd tea.Cmd
		switch c.foco {
		case cAsunto:
			c.asunto, cmd = c.asunto.Update(m)
		case cMensaje:
			if !c.previa {
				c.area, cmd = c.area.Update(m)
			}
		}
		c.guardarBorrador(a)
		return cmd
	case tickEnvio:
		if m.c == c && c.enviando {
			return tickEnvioCmd(c)
		}
	case publicadoMsg:
		if m.c != c {
			return nil
		}
		c.enviando = false
		if m.err != nil {
			if errors.Is(m.err, site.ErrSesion) {
				a.cab.Conectado = false
				c.err = T("sesion_vencida")
				return nil
			}
			c.err = textoError(m.err)
			return nil
		}
		delete(a.borradores, c.clave)
		a.guardarBorradores()
		if m.pub.EnRevision {
			a.avisar(T("en_cola"))
		} else {
			a.avisar(T("publicado"))
		}
		if a.actual() != c {
			return nil // no debería pasar: mientras manda, no se puede salir
		}
		if c.respuesta && c.hilo != nil && a.estaEnPila(c.hilo) {
			a.Volver()
			return c.hilo.alPublicar(a, m.pub.Post)
		}
		return a.Reemplazar(nuevaHilo(m.pub.Hilo, m.pub.Post))
	default:
		// El cursor que titila y demás mensajes de los componentes.
		var c1, c2 tea.Cmd
		c.area, c1 = c.area.Update(msg)
		c.asunto, c2 = c.asunto.Update(msg)
		return tea.Batch(c1, c2)
	}
	return nil
}

func tickEnvioCmd(c *pantComposer) tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return tickEnvio{c} })
}

// publicar valida como el sitio y manda el formulario.
func (c *pantComposer) publicar(a *App) tea.Cmd {
	cuerpo := site.LimpiarTexto(c.area.Value())
	asunto := strings.Join(strings.Fields(site.LimpiarTexto(c.asunto.Value())), " ")
	c.err = ""
	switch {
	case !c.respuesta && c.seccion < 0:
		c.err = T("elegi_tablon")
		c.enfocar(cTablon)
	case !c.respuesta && asunto == "":
		c.err = T("falta_asunto")
		c.enfocar(cAsunto)
	case !c.respuesta && site.Largo(asunto) > site.MaxAsunto:
		c.err = T("largo_asunto", site.MaxAsunto)
		c.enfocar(cAsunto)
	case cuerpo == "":
		c.err = T("vacio")
		c.enfocar(cMensaje)
	case site.Largo(cuerpo) > site.MaxCuerpo:
		c.err = T("largo_cuerpo", site.MaxCuerpo)
		c.enfocar(cMensaje)
	}
	if c.err != "" {
		return nil
	}
	c.enviando = true
	c.desde = time.Now()
	respuesta, hilo, sage := c.respuesta, c.hiloNo, c.sage
	seccion := ""
	if c.seccion >= 0 {
		seccion = site.Secciones[c.seccion].Slug
	}
	return tea.Batch(tickEnvioCmd(c), func() tea.Msg {
		// El filtro del sitio tarda unos segundos (a veces bastantes).
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
		defer cancel()
		var pub *site.Publicado
		var err error
		if respuesta {
			pub, err = a.cliente.Responder(ctx, hilo, cuerpo, sage)
		} else {
			pub, err = a.cliente.Publicar(ctx, seccion, asunto, cuerpo)
		}
		return publicadoMsg{c, pub, err}
	})
}

// ─── Dibujo ─────────────────────────────────────────────────────────────────────────────────

func (c *pantComposer) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	es := a.es
	t := a.tema
	var ls []Linea
	if c.respuesta {
		ls = append(ls, a.h1(T("responder"), w)...)
		ls = append(ls, txt(es.tenue, recortar(T("en", c.asuntoHilo), w)))
	} else {
		titulo := T("publicar")
		if c.seccion >= 0 {
			titulo = T("publicar_en", site.Secciones[c.seccion].Nombre)
		}
		ls = append(ls, a.h1(titulo, w)...)
	}
	if c.err != "" {
		for _, l := range a.parrafo(T("error")+c.err, w, es.error.negrita()) {
			ls = append(ls, l)
		}
	}
	etiqueta := func(s string, foco bool) Linea {
		e := es.tenue
		if foco {
			e = es.acento
		}
		return txt(e, mayus(s))
	}
	// Tablón (solo al publicar): ◀ Juegos ▶ y los demás al lado.
	if !c.respuesta {
		y := len(ls)
		l := Linea{etiqueta(T("tablon"), c.foco == cTablon)[0], {es.fondo, "  "}}
		x := l.Ancho()
		for i, s := range site.Secciones {
			e := es.acento
			if i == c.seccion {
				e = es.invertido
			}
			nombre := " " + s.Nombre + " "
			l = append(l, Run{e, nombre}, Run{es.fondo, " "})
			idx := i
			z.agregar(x, y, x+ancho(nombre), y+1, func() tea.Cmd { c.seccion = idx; c.enfocar(cAsunto); return nil })
			x += ancho(nombre) + 1
		}
		if c.foco == cTablon && c.seccion < 0 {
			l = append(l, Run{es.tenue, " ← " + T("elegi_tablon")})
		}
		ls = append(ls, envolverLinea(l, w)...)
		// Asunto: una caja de una línea.
		ls = append(ls, etiqueta(T("asunto"), c.foco == cAsunto))
		c.asunto.SetWidth(max(10, w-4))
		eb := es.borde
		if c.foco == cAsunto {
			eb = es.acento
		}
		ya := len(ls)
		ls = append(ls, caja([]Linea{mas(Linea{espacio(es.fondo, 1)}, Linea{crudo(c.asunto.View())})}, w, bordeSimple, eb, es.fondo)...)
		z.agregar(0, ya, w, ya+3, func() tea.Cmd { c.enfocar(cAsunto); return nil })
		ls = append(ls, etiqueta(T("mensaje"), c.foco == cMensaje))
	}
	// Lo de abajo: botones, sage y la ayuda de códigos.
	var abajo []Linea
	yBotones := 0
	botones := []Linea{
		a.boton(T("publicar"), c.foco == cPublicar),
		a.secundario(T("vista_previa"), c.foco == cPrevia),
		a.secundario(T("cancelar"), c.foco == cCancelar),
	}
	if c.previa {
		botones[1] = a.secundario(T("editar"), c.foco == cPrevia)
	}
	if c.enviando {
		frames := []string{"|", "/", "-", "\\"}
		f := frames[int(time.Since(c.desde)/(120*time.Millisecond))%len(frames)]
		botones[0] = txt(es.invertido, "[ "+mayus(T("revisando"))+" "+f+" ]")
	}
	zb := &Zonas{}
	fila := a.filaBotones(zb, 0, botones, []func() tea.Cmd{
		func() tea.Cmd { return c.publicar(a) },
		func() tea.Cmd { c.alternarPrevia(); return nil },
		func() tea.Cmd { return c.cancelar(a) },
	})
	n := site.Largo(c.area.Value())
	contE := es.tenue
	if n > site.MaxCuerpo*9/10 {
		contE = es.aviso
	}
	if n > site.MaxCuerpo {
		contE = es.error.negrita()
	}
	contador := txt(contE, miles(n)+" / "+miles(site.MaxCuerpo))
	abajo = append(abajo, aDerecha(fila, contador, w, es.fondo))
	if c.respuesta {
		marca := "[ ]"
		if c.sage {
			marca = "[x]"
		}
		e := es.tenue
		if c.foco == cSage {
			e = es.acento
		}
		abajo = append(abajo, Linea{{es.acento, marca}, {e, " " + T("sage")}})
	}
	ayuda := a.parrafo(T("ayuda_codigos")+" · "+T("atajo_publicar"), w, es.tenue)
	abajo = append(abajo, ayuda...)

	// El cuadro del mensaje ocupa lo que queda.
	altoCaja := max(3, alto-len(ls)-len(abajo))
	yCaja := len(ls)
	if c.previa {
		var interior []Linea
		interior = append(interior, txt(Est{Bg: t.Caja, Fg: t.Tenue}, T("previa_nota")))
		if !c.respuesta && c.asunto.Value() != "" {
			interior = append(interior, txt(Est{Bg: t.Caja, Fg: t.Fosforo, Negrita: true}, c.asunto.Value()))
		}
		interior = append(interior, envolver(site.Formatear(c.area.Value()), w-4, a.textoSobre(t.Caja), true)...)
		interior = cortarConElipsis(interior, altoCaja-2, w-4)
		for i := range interior {
			interior[i] = mas(Linea{espacio(Est{Bg: t.Caja}, 1)}, interior[i])
		}
		for len(interior) < altoCaja-2 {
			interior = append(interior, nil)
		}
		ls = append(ls, caja(interior, w, bordePunteado, es.acento, Est{Bg: t.Caja})...)
	} else {
		c.area.SetWidth(max(10, w-4))
		c.area.SetHeight(altoCaja - 2)
		eb := es.borde
		if c.foco == cMensaje {
			eb = es.acento
		}
		vista := strings.Split(c.area.View(), "\n")
		var interior []Linea
		for i := 0; i < altoCaja-2; i++ {
			l := Linea{espacio(es.fondo, 1)}
			if i < len(vista) {
				l = append(l, crudo(vista[i]))
			}
			interior = append(interior, l)
		}
		ls = append(ls, caja(interior, w, bordeSimple, eb, es.fondo)...)
	}
	z.agregar(0, yCaja, w, yCaja+altoCaja, func() tea.Cmd { c.enfocar(cMensaje); return nil })
	yBotones = len(ls)
	z.sumar(zb, 0, yBotones)
	if c.respuesta {
		z.agregar(0, yBotones+1, 4+ancho(T("sage")), yBotones+2, func() tea.Cmd {
			c.sage = !c.sage
			c.enfocar(cSage)
			return nil
		})
	}
	ls = append(ls, abajo...)
	return rellenar(ls, alto)
}
