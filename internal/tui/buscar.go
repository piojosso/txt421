package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// pantBuscar: el cuadro de búsqueda y los resultados, en páginas.
type pantBuscar struct {
	input      textinput.Model
	enInput    bool
	consulta   string
	resultados []site.Resultado
	total      int
	sitioPag   int
	sitioTotal int
	cargando   bool
	err        error
	pedido     int
	sel        int
	paginas    []rango
	iniciado   bool
}

type buscarMsg struct {
	p      *pantBuscar
	pedido int
	b      *site.Busqueda
	err    error
}

func nuevaBuscar(consulta string) *pantBuscar {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 100
	ti.Placeholder = T("palabras")
	ti.SetVirtualCursor(true)
	ti.SetValue(consulta)
	ti.CursorEnd()
	p := &pantBuscar{input: ti, enInput: consulta == "", consulta: consulta}
	if p.enInput {
		p.input.Focus()
	}
	return p
}

func (p *pantBuscar) Titulo() string        { return T("buscar") }
func (p *pantBuscar) SeccionActiva() string { return "" }

func (p *pantBuscar) estilar(a *App) {
	t := a.tema
	base := lipgloss.NewStyle().Background(t.Fondo).Foreground(t.Texto)
	s := p.input.Styles()
	for _, st := range []*textinput.StyleState{&s.Focused, &s.Blurred} {
		st.Text, st.Prompt, st.Suggestion = base, base, base
		st.Placeholder = base.Foreground(t.Tenue)
	}
	s.Cursor.Color = t.Fosforo
	p.input.SetStyles(s)
}

func (p *pantBuscar) Recargar(a *App) tea.Cmd {
	if !p.iniciado {
		p.estilar(a)
		p.iniciado = true
	}
	if p.consulta == "" {
		return nil
	}
	p.resultados, p.sitioPag, p.err = nil, 0, nil
	return p.cargar(a, 1)
}

func (p *pantBuscar) cargar(a *App, pagina int) tea.Cmd {
	p.cargando = true
	p.pedido++
	pedido, q := p.pedido, p.consulta
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		b, err := a.cliente.Buscar(ctx, q, pagina)
		return buscarMsg{p, pedido, b, err}
	}
}

func (p *pantBuscar) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case buscarMsg:
		if m.p != p || m.pedido != p.pedido {
			return nil
		}
		p.cargando = false
		if m.err != nil {
			p.err = m.err
			return nil
		}
		a.actualizarCabecera(m.b.Cabecera)
		p.resultados = append(p.resultados, m.b.Resultados...)
		p.total, p.sitioPag, p.sitioTotal = m.b.Total, m.b.Pagina, m.b.Paginas
		if p.resultados == nil {
			p.resultados = []site.Resultado{}
		}
	case tea.PasteMsg:
		if p.enInput {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(m)
			return cmd
		}
	default:
		if p.enInput {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return cmd
		}
	}
	return nil
}

func (p *pantBuscar) Teclas(a *App) []Atajo {
	if p.enInput {
		return []Atajo{{"Enter", T("buscar")}, {"↓", T("k_mover")}, {"Esc", T("k_volver")}}
	}
	return []Atajo{{"↑↓", T("k_mover")}, {"←→", T("k_pagina")}, {"Enter", T("k_abrir")}, {"/", T("k_buscar")}, {"Esc", T("k_volver")}}
}

func (p *pantBuscar) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	s := k.String()
	if p.enInput {
		switch s {
		case "enter":
			q := strings.TrimSpace(p.input.Value())
			if q == "" {
				return true, nil
			}
			p.consulta = q
			p.sel = 0
			p.enInput = false
			p.input.Blur()
			return true, p.Recargar(a)
		case "esc":
			if len(p.resultados) > 0 {
				p.enInput = false
				p.input.Blur()
				return true, nil
			}
			return true, a.Volver()
		case "down", "tab":
			if len(p.resultados) > 0 {
				p.enInput = false
				p.input.Blur()
			}
			return true, nil
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(k)
		return true, cmd
	}
	switch s {
	case "/", "tab", "shift+tab":
		p.enInput = true
		return true, p.input.Focus()
	case "up", "k":
		if p.sel == 0 {
			p.enInput = true
			return true, p.input.Focus()
		}
		p.sel--
	case "down", "j":
		return true, p.mover(a, 1)
	case "right", "pgdown", "space":
		return true, p.pagina(a, 1)
	case "left", "pgup":
		return true, p.pagina(a, -1)
	case "enter":
		if p.sel < len(p.resultados) {
			r := p.resultados[p.sel]
			return true, a.Ir(nuevaHilo(r.Hilo, r.Post))
		}
	default:
		return false, nil
	}
	return true, nil
}

func (p *pantBuscar) mover(a *App, d int) tea.Cmd {
	p.sel = max(0, min(p.sel+d, len(p.resultados)-1))
	if p.sel >= len(p.resultados)-3 && p.sitioPag < p.sitioTotal && !p.cargando {
		return p.cargar(a, p.sitioPag+1)
	}
	return nil
}

func (p *pantBuscar) pagina(a *App, d int) tea.Cmd {
	if len(p.paginas) == 0 {
		return nil
	}
	pag := paginaDe(p.paginas, p.sel) + d
	if pag < 0 {
		pag = 0
	}
	if pag >= len(p.paginas) {
		return p.mover(a, len(p.resultados))
	}
	p.sel = p.paginas[pag].desde
	return p.mover(a, 0)
}

func (p *pantBuscar) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	es := a.es
	ls := a.h1(T("buscar"), w)
	// Cuadro de búsqueda + [BUSCAR].
	b := a.boton(T("buscar"), false)
	p.input.SetWidth(max(10, w-b.Ancho()-5))
	eb := es.borde
	if p.enInput {
		eb = es.acento
	}
	y := len(ls)
	cuadro := caja([]Linea{mas(Linea{espacio(es.fondo, 1)}, Linea{crudo(p.input.View())})}, w-b.Ancho()-1, bordeSimple, eb, es.fondo)
	for i, l := range cuadro {
		if i == 1 {
			l = mas(l, Linea{espacio(es.fondo, 1)}, b)
		}
		ls = append(ls, l)
	}
	z.agregar(0, y, w-b.Ancho()-1, y+3, func() tea.Cmd { p.enInput = true; return p.input.Focus() })
	z.agregar(w-b.Ancho(), y+1, w, y+2, func() tea.Cmd { return p.Tecla2(a, "enter") })
	if p.consulta == "" {
		return rellenar(ls, alto)
	}
	if p.resultados == nil {
		return append(ls, a.estadoCarga(w, alto-len(ls), p.err)...)
	}
	if len(p.resultados) == 0 {
		return rellenar(append(ls, txt(es.tenue, T("nada"))), alto)
	}
	total := T("resultados_n", p.total)
	if p.total == 1 {
		total = T("resultados_1")
	}
	ls = append(ls, txt(es.tenue, total))
	disponible := alto - len(ls) - 2
	bloques := make([][]Linea, len(p.resultados))
	alturas := make([]int, len(p.resultados))
	for i, r := range p.resultados {
		bloques[i] = p.dibujarResultado(a, r, w, i == p.sel && !p.enInput)
		alturas[i] = len(bloques[i])
	}
	p.paginas = paginarAlturas(alturas, disponible, 0)
	pag := paginaDe(p.paginas, p.sel)
	rg := p.paginas[pag]
	var cuerpo []Linea
	y0 := len(ls)
	for i := rg.desde; i < rg.hasta; i++ {
		idx := i
		yy := y0 + len(cuerpo)
		z.agregar(0, yy, w, yy+len(bloques[i]), func() tea.Cmd {
			if p.sel == idx && !p.enInput {
				r := p.resultados[idx]
				return a.Ir(nuevaHilo(r.Hilo, r.Post))
			}
			p.sel, p.enInput = idx, false
			p.input.Blur()
			return nil
		})
		cuerpo = append(cuerpo, bloques[i]...)
	}
	ls = append(ls, rellenar(cuerpo, disponible)...)
	ls = append(ls, nil)
	totalPags := len(p.paginas)
	if p.sitioPag < p.sitioTotal {
		totalPags += (p.sitioTotal - p.sitioPag) * max(1, len(p.paginas)/max(1, p.sitioPag))
	}
	yp := len(ls)
	ls = append(ls, a.paginacion(z, yp, pag+1, totalPags, w, func(n int) tea.Cmd { return p.pagina(a, n-1-pag) }))
	return rellenar(ls, alto)
}

// Tecla2 simula una tecla (para los clicks).
func (p *pantBuscar) Tecla2(a *App, s string) tea.Cmd {
	if s == "enter" {
		q := strings.TrimSpace(p.input.Value())
		if q == "" {
			return nil
		}
		p.consulta, p.sel, p.enInput = q, 0, false
		p.input.Blur()
		return p.Recargar(a)
	}
	return nil
}

// dibujarResultado: .resultados li (línea punteada arriba, asunto, datos, fragmento).
func (p *pantBuscar) dibujarResultado(a *App, r site.Resultado, w int, sel bool) []Linea {
	es := a.es
	ls := []Linea{txt(es.borde, strings.Repeat("┄", w))}
	e := es.acento.negrita()
	if sel {
		e = es.invertido
	}
	ls = append(ls, txt(e, recortar(r.Asunto, w)))
	ls = append(ls, txt(es.tenue, recortar(r.Info, w)))
	frag := envolver(r.Fragmento, w, a.textoSobre(a.tema.Fondo), false)
	ls = append(ls, cortarConElipsis(frag, 3, w)...)
	return ls
}
