package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/piojosso/txt421/internal/config"
	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// ventana dibuja una caja con fondo de caja y borde de fósforo (.menu-desplegable del sitio).
func (a *App) ventana(interior []Linea, w int) []Linea {
	t := a.tema
	base := Est{Bg: t.Caja, Fg: t.Texto}
	for i := range interior {
		interior[i] = mas(Linea{espacio(base, 1)}, completar(interior[i], w-4, base), Linea{espacio(base, 1)})
	}
	return caja(interior, w, bordeSimple, Est{Fg: t.Fosforo, Bg: t.Caja}, base)
}

// ─── Menú (☰) ───────────────────────────────────────────────────────────────────────────────

type itemMenu struct {
	texto  string
	accion func() tea.Cmd
}

type capaMenu struct {
	items []itemMenu
	sel   int
}

func nuevoMenu(a *App) *capaMenu {
	var its []itemMenu
	if a.conectado() {
		r := T("respuestas")
		if a.cab.Novedades > 0 {
			r += " (" + itoa(a.cab.Novedades) + ")"
		}
		its = append(its,
			itemMenu{r, func() tea.Cmd { return a.Ir(nuevaBandeja()) }},
			itemMenu{T("guardados"), func() tea.Cmd { return a.Ir(nuevaGuardados()) }},
		)
	} else {
		its = append(its, itemMenu{T("entrar"), func() tea.Cmd { return a.Ir(nuevaEntrar()) }})
	}
	its = append(its,
		itemMenu{T("preferencias"), func() tea.Cmd { return a.Ir(nuevaPreferencias()) }},
		itemMenu{T("normas"), func() tea.Cmd { return a.Ir(nuevaDocumento("/normas", T("normas"))) }},
		itemMenu{T("formato"), func() tea.Cmd { return a.Ir(nuevaDocumento("/formato", T("formato"))) }},
		itemMenu{T("abrir_web"), func() tea.Cmd { abrirNavegador(site.BaseURL); return nil }},
	)
	if a.nueva != "" && a.op.Actualizar != nil {
		its = append(its, itemMenu{T("actualizar") + " → " + a.nueva, func() tea.Cmd {
			p := nuevaPreferencias()
			c := a.Ir(p)
			return tea.Batch(c, p.actualizar(a))
		}})
	}
	if a.conectado() {
		its = append(its, itemMenu{T("salir"), func() tea.Cmd {
			a.capa = nuevoConfirmar(T("confirmar_salir"), T("salir"), func() tea.Cmd { return a.salirDeCuenta() })
			return nil
		}})
	}
	its = append(its, itemMenu{T("cerrar_app"), func() tea.Cmd { return a.Salir() }})
	return &capaMenu{items: its}
}

func (c *capaMenu) Teclas() []Atajo {
	return []Atajo{{"↑↓", T("k_mover")}, {"Enter", T("k_abrir")}, {"Esc", T("k_volver")}}
}

func (c *capaMenu) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	w := 4
	for _, it := range c.items {
		w = max(w, ancho(it.texto)+6)
	}
	var in []Linea
	t := a.tema
	for i, it := range c.items {
		e := Est{Bg: t.Caja, Fg: t.Fosforo}
		if i == c.sel {
			e = a.es.invertido
		}
		in = append(in, txt(e, " "+it.texto+strings.Repeat(" ", max(0, w-6-ancho(it.texto)))+" "))
		idx := i
		z.agregar(0, i+1, w, i+2, func() tea.Cmd { a.capa = nil; return c.items[idx].accion() })
	}
	ls := a.ventana(in, w)
	anchoCont := min(W-2, maxContenido)
	x := (W-anchoCont)/2 + anchoCont - w
	return ls, max(0, x), 1
}

func (c *capaMenu) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "up", "k", "shift+tab":
		c.sel = (c.sel - 1 + len(c.items)) % len(c.items)
	case "down", "j", "tab":
		c.sel = (c.sel + 1) % len(c.items)
	case "enter", "space", "right":
		a.capa = nil
		return c.items[c.sel].accion()
	case "esc", "m", "q", "left", "backspace":
		a.capa = nil
	}
	return nil
}

func (c *capaMenu) Mensaje(a *App, msg tea.Msg) tea.Cmd { return nil }

// ─── Ayuda (?) ──────────────────────────────────────────────────────────────────────────────

type capaAyuda struct{}

func (c *capaAyuda) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	es := a.es
	t := a.tema
	base := Est{Bg: t.Caja, Fg: t.Texto}
	tecla := Est{Bg: t.Caja, Fg: t.Fosforo, Negrita: true}
	tit := Est{Bg: t.Caja, Fg: t.Tenue}
	grupos := []struct {
		titulo string
		teclas [][2]string
	}{
		{T("portada"), [][2]string{
			{"↑↓←→ · clic", T("k_mover")}, {"Enter", T("k_abrir")}, {"PgUp PgDn · [ ]", T("k_pagina")},
			{"Tab · 0-5", T("k_seccion")}, {"v", T("k_vista") + " (" + T("catalogo") + "/" + T("lista") + ")"},
			{"a", T("archivo")}, {"n", T("k_publicar")},
		}},
		{"", [][2]string{
			{"↑↓", T("k_mover")}, {"←→ PgUp PgDn", T("k_pagina")}, {"Enter", T("k_citar") + " (" + T("responder") + " >>N)"},
			{"r", T("k_responder")}, {"i · Esc", T("k_ir_cita") + " · " + T("k_volver")}, {"s", "spoiler"},
			{"g", T("guardar")}, {"x", T("reportar") + " / " + T("borrar")}, {"w", T("abrir_web")},
		}},
		{T("publicar"), [][2]string{
			{"Tab", T("k_campo")}, {"Ctrl+Enter · Ctrl+S · F2", T("k_publicar")}, {"Ctrl+P · F3", T("k_previa")}, {"Esc", T("k_volver")},
		}},
		{"", [][2]string{
			{"/", T("k_buscar")}, {"e", T("respuestas")}, {"m · F10", T("k_menu")}, {"t", T("k_tema")},
			{"r · F5", "↻"}, {"q · Ctrl+C", T("k_salir")},
		}},
	}
	grupos[1].titulo = strings.TrimSuffix(T("responder"), "") + " / " + T("guardar")
	grupos[3].titulo = "txt421"
	w := min(W-4, 64)
	var in []Linea
	in = append(in, txt(Est{Bg: t.Caja, Fg: t.Fosforo, Negrita: true}, "> "+mayus(T("ayuda_titulo"))))
	for _, g := range grupos {
		in = append(in, nil, txt(tit, mayus(g.titulo)))
		for _, k := range g.teclas {
			in = append(in, Linea{{tecla, k[0] + strings.Repeat(" ", max(1, 26-ancho(k[0])))}, {base, k[1]}})
		}
	}
	in = append(in, nil, txt(tit, T("ayuda_cerrar")))
	if len(in) > H-4 {
		in = in[:max(1, H-4)]
	}
	_ = es
	ls := a.ventana(in, w)
	return ls, (W - w) / 2, max(0, (H-len(ls))/2)
}

func (c *capaAyuda) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	a.capa = nil
	return nil
}

func (c *capaAyuda) Mensaje(a *App, msg tea.Msg) tea.Cmd { return nil }

// ─── Confirmar ──────────────────────────────────────────────────────────────────────────────

type capaConfirmar struct {
	pregunta, si string
	accion       func() tea.Cmd
	sel          int
}

func nuevoConfirmar(pregunta, si string, accion func() tea.Cmd) *capaConfirmar {
	return &capaConfirmar{pregunta: pregunta, si: si, accion: accion}
}

func (c *capaConfirmar) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	w := min(W-4, max(40, ancho(c.pregunta)+6))
	base := Est{Bg: a.tema.Caja, Fg: a.tema.Texto}
	in := envolver([]site.Tramo{{Texto: c.pregunta}}, w-4, estilosTexto{normal: base}, false)
	in = append(in, nil)
	b1 := a.boton(c.si, c.sel == 0)
	b2 := a.secundario(T("cancelar"), c.sel == 1)
	y := len(in) + 1
	z.agregar(2, y, 2+b1.Ancho(), y+1, func() tea.Cmd { a.capa = nil; return c.accion() })
	z.agregar(3+b1.Ancho(), y, 3+b1.Ancho()+b2.Ancho(), y+1, func() tea.Cmd { a.capa = nil; return nil })
	in = append(in, mas(b1, Linea{{base, " "}}, b2))
	ls := a.ventana(in, w)
	return ls, (W - w) / 2, max(0, (H-len(ls))/2)
}

func (c *capaConfirmar) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "left", "right", "tab", "shift+tab", "h", "l":
		c.sel = 1 - c.sel
	case "enter", "space":
		a.capa = nil
		if c.sel == 0 {
			return c.accion()
		}
	case "esc", "n", "q":
		a.capa = nil
	}
	return nil
}

func (c *capaConfirmar) Mensaje(a *App, msg tea.Msg) tea.Cmd { return nil }

// ─── Reportar ───────────────────────────────────────────────────────────────────────────────

type dialogoReportar struct {
	hilo    *pantHilo
	post    site.Post
	motivos []site.Motivo
	sel     int
	foco    int // 0 motivos, 1 enviar, 2 cancelar
	err     string
	ocupado bool
	cargado bool
}

type motivosMsg struct {
	d   *dialogoReportar
	ms  []site.Motivo
	err error
}
type reportadoMsg struct {
	d   *dialogoReportar
	err error
}

func nuevoDialogoReportar(a *App, h *pantHilo, p site.Post) *dialogoReportar {
	return &dialogoReportar{hilo: h, post: p}
}

func (d *dialogoReportar) cargar(a *App) tea.Cmd {
	no := d.post.No
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		ms, err := a.cliente.Motivos(ctx, no)
		return motivosMsg{d, ms, err}
	}
}

func (d *dialogoReportar) Teclas() []Atajo {
	return []Atajo{{"↑↓", T("k_elegir")}, {"Tab", T("k_campo")}, {"Enter", T("enviar_reporte")}, {"Esc", T("cancelar")}}
}

func (d *dialogoReportar) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case motivosMsg:
		if m.d != d {
			return nil
		}
		d.cargado = true
		if m.err != nil {
			d.err = textoError(m.err)
		}
		d.motivos = m.ms
	case reportadoMsg:
		if m.d != d {
			return nil
		}
		d.ocupado = false
		if m.err != nil {
			d.err = textoError(m.err)
			return nil
		}
		a.capa = nil
		a.avisar(T("reportado"))
		return d.hilo.Recargar(a)
	}
	return nil
}

func (d *dialogoReportar) enviar(a *App) tea.Cmd {
	if len(d.motivos) == 0 || d.ocupado {
		return nil
	}
	d.ocupado = true
	no, motivo := d.post.No, d.motivos[d.sel].ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return reportadoMsg{d, a.cliente.Reportar(ctx, no, motivo)}
	}
}

func (d *dialogoReportar) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc", "q":
		a.capa = nil
	case "tab", "right":
		d.foco = (d.foco + 1) % 3
	case "shift+tab", "left":
		d.foco = (d.foco + 2) % 3
	case "up", "k":
		if d.foco == 0 && d.sel > 0 {
			d.sel--
		}
	case "down", "j":
		if d.foco == 0 && d.sel < len(d.motivos)-1 {
			d.sel++
		}
	case "enter", "space":
		if d.foco == 2 {
			a.capa = nil
			return nil
		}
		return d.enviar(a)
	}
	return nil
}

func (d *dialogoReportar) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	t := a.tema
	w := min(W-4, 72)
	iw := w - 4
	base := Est{Bg: t.Caja, Fg: t.Texto}
	tenue := base.conFg(t.Tenue)
	in := []Linea{txt(base.conFg(t.Fosforo).negrita(), "> "+mayus(T("reportar")))}
	in = append(in, envolver([]site.Tramo{{Texto: T("en", d.hilo.hilo.Asunto) + ". " + T("reportar_ayuda")}}, iw, estilosTexto{normal: tenue}, false)...)
	in = append(in, nil)
	in = append(in, a.dibujarPost(d.post, iw, opcionesPost{maxTexto: 3})...)
	in = append(in, nil, txt(tenue, mayus(T("motivo"))))
	if !d.cargado {
		in = append(in, txt(tenue, T("cargando")))
	}
	y0 := len(in) + 1
	for i, m := range d.motivos {
		e := base.conFg(t.Fosforo)
		marca := "( ) "
		if i == d.sel {
			marca = "(•) "
			if d.foco == 0 {
				e = a.es.invertido
			}
		}
		in = append(in, txt(e, marca+m.Titulo))
		idx := i
		z.agregar(2, y0+i, w-2, y0+i+1, func() tea.Cmd { d.sel, d.foco = idx, 0; return nil })
	}
	if d.err != "" {
		in = append(in, txt(base.conFg(t.Error).negrita(), recortar(T("error")+d.err, iw)))
	}
	in = append(in, nil)
	envio := T("enviar_reporte")
	if d.ocupado {
		envio = T("revisando")
	}
	b1 := a.boton(envio, d.foco == 1)
	b2 := a.secundario(T("cancelar"), d.foco == 2)
	yb := len(in) + 1
	z.agregar(2, yb, 2+b1.Ancho(), yb+1, func() tea.Cmd { d.foco = 1; return d.enviar(a) })
	z.agregar(3+b1.Ancho(), yb, 3+b1.Ancho()+b2.Ancho(), yb+1, func() tea.Cmd { a.capa = nil; return nil })
	in = append(in, mas(b1, Linea{{base, " "}}, b2))
	if len(in) > H-2 {
		in = in[len(in)-(H-2):]
	}
	ls := a.ventana(in, w)
	return ls, (W - w) / 2, max(0, (H-len(ls))/2)
}

// ─── Borrar ─────────────────────────────────────────────────────────────────────────────────

type dialogoBorrar struct {
	hilo    *pantHilo
	post    site.Post
	motivo  string // por qué no se puede todavía
	cargado bool
	sel     int
	err     string
	ocupado bool
}

type puedeBorrarMsg struct {
	d      *dialogoBorrar
	motivo string
	err    error
}
type borradoMsg struct {
	d   *dialogoBorrar
	err error
}

func nuevoDialogoBorrar(a *App, h *pantHilo, p site.Post) *dialogoBorrar {
	return &dialogoBorrar{hilo: h, post: p, sel: 1} // "Cancelar" elegido por defecto
}

func (d *dialogoBorrar) cargar(a *App) tea.Cmd {
	no := d.post.No
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		m, err := a.cliente.PuedeBorrar(ctx, no)
		return puedeBorrarMsg{d, m, err}
	}
}

func (d *dialogoBorrar) Teclas() []Atajo {
	return []Atajo{{"←→", T("k_elegir")}, {"Enter", T("k_elegir")}, {"Esc", T("cancelar")}}
}

func (d *dialogoBorrar) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case puedeBorrarMsg:
		if m.d != d {
			return nil
		}
		d.cargado = true
		d.motivo = m.motivo
		if m.err != nil {
			d.err = textoError(m.err)
		}
	case borradoMsg:
		if m.d != d {
			return nil
		}
		d.ocupado = false
		if m.err != nil {
			d.err = textoError(m.err)
			return nil
		}
		a.capa = nil
		a.avisar(T("borrado"))
		return d.hilo.Recargar(a)
	}
	return nil
}

func (d *dialogoBorrar) borrar(a *App) tea.Cmd {
	if !d.cargado || d.motivo != "" || d.ocupado || d.err != "" {
		return nil
	}
	d.ocupado = true
	no := d.post.No
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return borradoMsg{d, a.cliente.Borrar(ctx, no)}
	}
}

func (d *dialogoBorrar) Tecla(a *App, k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc", "q", "n":
		a.capa = nil
	case "left", "right", "tab", "shift+tab", "h", "l":
		d.sel = 1 - d.sel
	case "enter", "space":
		if d.sel == 1 || d.motivo != "" {
			a.capa = nil
			return nil
		}
		return d.borrar(a)
	}
	return nil
}

func (d *dialogoBorrar) Dibujar(a *App, z *Zonas, W, H int) ([]Linea, int, int) {
	t := a.tema
	w := min(W-4, 72)
	iw := w - 4
	base := Est{Bg: t.Caja, Fg: t.Texto}
	tenue := base.conFg(t.Tenue)
	ayuda := T("borrar_ayuda")
	if d.post.Inicial {
		ayuda = T("borrar_op")
	}
	in := []Linea{txt(base.conFg(t.Fosforo).negrita(), "> "+mayus(T("borrar")))}
	in = append(in, envolver([]site.Tramo{{Texto: T("en", d.hilo.hilo.Asunto) + ". " + ayuda}}, iw, estilosTexto{normal: tenue}, false)...)
	in = append(in, nil)
	in = append(in, a.dibujarPost(d.post, iw, opcionesPost{maxTexto: 3})...)
	in = append(in, nil)
	switch {
	case !d.cargado:
		in = append(in, txt(tenue, T("cargando")))
	case d.motivo != "":
		in = append(in, envolver([]site.Tramo{{Texto: d.motivo}}, iw, estilosTexto{normal: base.conFg(t.Aviso)}, false)...)
		in = append(in, nil)
		b := a.boton(T("volver"), true)
		y := len(in) + 1
		z.agregar(2, y, 2+b.Ancho(), y+1, func() tea.Cmd { a.capa = nil; return nil })
		in = append(in, b)
	default:
		in = append(in, txt(tenue, T("no_deshacer")))
		if d.err != "" {
			in = append(in, txt(base.conFg(t.Error).negrita(), recortar(T("error")+d.err, iw)))
		}
		si := T("si_borrar")
		if d.ocupado {
			si = T("revisando")
		}
		b1 := a.boton(si, d.sel == 0)
		b2 := a.secundario(T("cancelar"), d.sel == 1)
		y := len(in) + 1
		z.agregar(2, y, 2+b1.Ancho(), y+1, func() tea.Cmd { d.sel = 0; return d.borrar(a) })
		z.agregar(3+b1.Ancho(), y, 3+b1.Ancho()+b2.Ancho(), y+1, func() tea.Cmd { a.capa = nil; return nil })
		in = append(in, mas(b1, Linea{{base, " "}}, b2))
	}
	ls := a.ventana(in, w)
	return ls, (W - w) / 2, max(0, (H-len(ls))/2)
}

var _ = config.Dir
