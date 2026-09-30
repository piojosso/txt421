package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// ─── Lista genérica de publicaciones (Respuestas y Guardados) ──────────────────────────────

type itemLista struct {
	hilo, post int
	asunto     string
	info       string
	extracto   string
	nueva      bool
	seccion    string // título de sección dentro de la lista ("Donde participaste")
}

// pantLista: /respuestas o /guardados.
type pantLista struct {
	tipo    string // "respuestas" o "guardados"
	items   []itemLista
	cargado bool
	err     error
	pedido  int
	sel     int
	paginas []rango
	sinMias bool
}

type listaMsg struct {
	p       *pantLista
	pedido  int
	items   []itemLista
	cab     site.Cabecera
	sinMias bool
	err     error
}

func nuevaBandeja() *pantLista   { return &pantLista{tipo: "respuestas"} }
func nuevaGuardados() *pantLista { return &pantLista{tipo: "guardados"} }

func (p *pantLista) Titulo() string        { return T(p.tipo) }
func (p *pantLista) SeccionActiva() string { return "" }

func (p *pantLista) Recargar(a *App) tea.Cmd {
	p.err = nil
	p.pedido++
	pedido, tipo := p.pedido, p.tipo
	titulo := T("donde_participaste")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		var items []itemLista
		if tipo == "respuestas" {
			b, err := a.cliente.Bandeja(ctx)
			if err != nil {
				return listaMsg{p: p, pedido: pedido, err: err}
			}
			for _, v := range b.Avisos {
				items = append(items, itemLista{hilo: v.Hilo, post: v.Post, asunto: v.Asunto, info: v.Info, extracto: v.Extracto, nueva: v.Nueva})
			}
			sinMias := len(b.Mias) == 0
			for i, e := range b.Mias {
				it := itemLista{hilo: e.Hilo, asunto: e.Asunto, info: e.Info}
				if i == 0 {
					it.seccion = titulo
				}
				items = append(items, it)
			}
			// Después de verla, el sitio ya las dio por leídas.
			b.Cabecera.Novedades = 0
			return listaMsg{p, pedido, items, b.Cabecera, sinMias, nil}
		}
		g, err := a.cliente.Guardados(ctx)
		if err != nil {
			return listaMsg{p: p, pedido: pedido, err: err}
		}
		for _, e := range g.Lista {
			items = append(items, itemLista{hilo: e.Hilo, asunto: e.Asunto, info: e.Info})
		}
		return listaMsg{p, pedido, items, g.Cabecera, true, nil}
	}
}

func (p *pantLista) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	m, ok := msg.(listaMsg)
	if !ok || m.p != p || m.pedido != p.pedido {
		return nil
	}
	if m.err != nil {
		if errors.Is(m.err, site.ErrSesion) {
			a.cab.Conectado = false
			if a.actual() == p {
				return a.Reemplazar(nuevaEntrar())
			}
			return nil
		}
		p.err = m.err
		return nil
	}
	a.actualizarCabecera(m.cab)
	p.items, p.cargado, p.sinMias = m.items, true, m.sinMias
	p.sel = min(p.sel, max(0, len(p.items)-1))
	return nil
}

func (p *pantLista) Teclas(a *App) []Atajo {
	return []Atajo{{"↑↓", T("k_mover")}, {"←→", T("k_pagina")}, {"Enter", T("k_abrir")}, {"r", "↻"}, {"Esc", T("k_volver")}}
}

func (p *pantLista) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	switch k.String() {
	case "up", "k":
		p.sel = max(0, p.sel-1)
	case "down", "j":
		p.sel = min(max(0, len(p.items)-1), p.sel+1)
	case "right", "pgdown", "space":
		p.saltar(1)
	case "left", "pgup":
		p.saltar(-1)
	case "enter":
		if p.sel < len(p.items) {
			it := p.items[p.sel]
			return true, a.Ir(nuevaHilo(it.hilo, it.post))
		}
	default:
		return false, nil
	}
	return true, nil
}

func (p *pantLista) saltar(d int) {
	if len(p.paginas) == 0 {
		return
	}
	pag := max(0, min(len(p.paginas)-1, paginaDe(p.paginas, p.sel)+d))
	p.sel = p.paginas[pag].desde
}

func (p *pantLista) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	es := a.es
	ls := a.h1(T(p.tipo), w)
	ayuda := T("respuestas_ayuda")
	if p.tipo == "guardados" {
		ayuda = T("guardados_ayuda")
	}
	ls = append(ls, a.parrafo(ayuda, w, es.tenue)...)
	ls = append(ls, nil)
	if !p.cargado {
		return append(ls, a.estadoCarga(w, alto-len(ls), p.err)...)
	}
	if len(p.items) == 0 || (p.tipo == "respuestas" && p.items[0].seccion != "") {
		vacio := T("sin_respuestas")
		if p.tipo == "guardados" {
			vacio = T("sin_guardados")
		}
		ls = append(ls, txt(es.texto, vacio))
	}
	if p.tipo == "respuestas" && p.sinMias {
		ls = append(ls, nil, a.h2(T("donde_participaste")), txt(es.tenue, T("sin_participar")))
	}
	disponible := alto - len(ls) - 2
	bloques := make([][]Linea, len(p.items))
	alturas := make([]int, len(p.items))
	for i, it := range p.items {
		bloques[i] = p.dibujarItem(a, it, w, i == p.sel)
		alturas[i] = len(bloques[i])
	}
	p.paginas = paginarAlturas(alturas, disponible, 0)
	pag := paginaDe(p.paginas, p.sel)
	var cuerpo []Linea
	y0 := len(ls)
	if len(p.items) > 0 {
		rg := p.paginas[pag]
		for i := rg.desde; i < rg.hasta; i++ {
			idx := i
			yy := y0 + len(cuerpo)
			z.agregar(0, yy, w, yy+len(bloques[i]), func() tea.Cmd {
				if p.sel == idx {
					it := p.items[idx]
					return a.Ir(nuevaHilo(it.hilo, it.post))
				}
				p.sel = idx
				return nil
			})
			cuerpo = append(cuerpo, bloques[i]...)
		}
	}
	ls = append(ls, rellenar(cuerpo, max(0, disponible))...)
	ls = append(ls, nil)
	if p.tipo == "respuestas" {
		ls = append(ls, txt(es.tenue, recortar(T("solo_vos"), w)))
	} else {
		ls = append(ls, a.paginacion(z, len(ls), pag+1, len(p.paginas), w, func(n int) tea.Cmd { p.saltar(n - 1 - pag); return nil }))
	}
	return rellenar(ls, alto)
}

// dibujarItem: .avisos li (borde a la izquierda; en aviso si es nueva) o .mias li.
func (p *pantLista) dibujarItem(a *App, it itemLista, w int, sel bool) []Linea {
	es := a.es
	var ls []Linea
	if it.seccion != "" {
		ls = append(ls, nil, a.h2(it.seccion))
	}
	barra := es.borde
	if it.nueva {
		barra = es.aviso
	}
	if sel {
		barra = es.acento
	}
	e := es.acento.subrayado()
	if sel {
		e = es.invertido
	}
	cuerpo := []Linea{txt(e, recortar(it.asunto, w-3))}
	info := it.info
	cuerpo = append(cuerpo, txt(es.tenue, recortar(info, w-3)))
	if it.extracto != "" {
		cuerpo = append(cuerpo, cortarConElipsis(a.parrafo(it.extracto, w-3, es.texto), 2, w-3)...)
	}
	for _, l := range cuerpo {
		ls = append(ls, mas(txt(barra, "│ "), l))
	}
	ls = append(ls, nil)
	return ls
}

// ─── Documentos (Normas, Formato) ───────────────────────────────────────────────────────────

type pantDocumento struct {
	ruta, titulo string
	doc          *site.Documento
	err          error
	pag          int
	total        int
}

type documentoMsg struct {
	p   *pantDocumento
	d   *site.Documento
	err error
}

func nuevaDocumento(ruta, titulo string) *pantDocumento {
	return &pantDocumento{ruta: ruta, titulo: titulo}
}

func (p *pantDocumento) Titulo() string        { return p.titulo }
func (p *pantDocumento) SeccionActiva() string { return "" }

func (p *pantDocumento) Recargar(a *App) tea.Cmd {
	ruta := p.ruta
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		d, err := a.cliente.Documento(ctx, ruta)
		return documentoMsg{p, d, err}
	}
}

func (p *pantDocumento) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	if m, ok := msg.(documentoMsg); ok && m.p == p {
		p.doc, p.err = m.d, m.err
	}
	return nil
}

func (p *pantDocumento) Teclas(a *App) []Atajo {
	return []Atajo{{"←→", T("k_pagina")}, {"Esc", T("k_volver")}}
}

func (p *pantDocumento) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	switch k.String() {
	case "right", "down", "pgdown", "space", "j":
		p.pag = min(p.pag+1, max(0, p.total-1))
	case "left", "up", "pgup", "k":
		p.pag = max(0, p.pag-1)
	default:
		return false, nil
	}
	return true, nil
}

func (p *pantDocumento) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	if p.doc == nil {
		return a.estadoCarga(w, alto, p.err)
	}
	es := a.es
	t := a.tema
	var ls []Linea
	ls = append(ls, a.h1(p.doc.Titulo, w)...)
	fondo := estilosTexto{normal: es.texto, marca: es.acento.negrita(), cita: es.cita, ref: es.cita.subrayado(),
		spoiler: Est{Fg: t.Fosforo, Bg: t.Fosforo}, spoilerVisto: es.texto.subrayado()}
	for _, b := range p.doc.Bloques {
		switch b.Tipo {
		case "h2":
			ls = append(ls, nil, a.h2(site.TextoPlano(b.Texto)))
		case "ayuda":
			f := fondo
			f.normal = es.tenue
			ls = append(ls, envolver(b.Texto, w, f, false)...)
		case "li":
			pre := itoa(b.Num) + ". "
			for i, l := range envolver(b.Texto, w-len(pre), fondo, false) {
				p0 := strings.Repeat(" ", len(pre))
				if i == 0 {
					p0 = pre
				}
				ls = append(ls, mas(txt(es.acento, p0), l))
			}
		case "pre":
			for _, l := range strings.Split(strings.TrimRight(site.TextoPlano(b.Texto), "\n"), "\n") {
				ls = append(ls, Linea{{es.borde, "┆ "}, {es.cita, l}})
			}
		default:
			ls = append(ls, envolver(b.Texto, w, fondo, true)...)
		}
	}
	// En páginas.
	porPag := max(1, alto-2)
	p.total = (len(ls) + porPag - 1) / porPag
	p.pag = min(p.pag, max(0, p.total-1))
	desde := p.pag * porPag
	pagina := ls[desde:min(len(ls), desde+porPag)]
	out := rellenar(append([]Linea{}, pagina...), porPag)
	out = append(out, nil)
	out = append(out, a.paginacion(z, len(out), p.pag+1, p.total, w, func(n int) tea.Cmd { p.pag = n - 1; return nil }))
	return rellenar(out, alto)
}
