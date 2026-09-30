package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/piojosso/txt421/internal/config"
	"github.com/piojosso/txt421/internal/entrar"
	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// pantEntrar: entrar con Google (abriendo un navegador) o pegando la cookie.
type pantEntrar struct {
	foco     int // 0 Google, 1 pegar
	pegando  bool
	input    textinput.Model
	estado   string
	err      string
	ocupado  bool
	cancelar context.CancelFunc
	listo    bool
}

type entrarMsg struct {
	p       *pantEntrar
	cookies map[string]string
	err     error
}
type verificadoMsg struct {
	p     *pantEntrar
	cab   site.Cabecera
	err   error
	antes map[string]string
}

func nuevaEntrar() *pantEntrar {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "sid"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 200
	ti.SetVirtualCursor(true)
	return &pantEntrar{input: ti}
}

func (p *pantEntrar) Titulo() string        { return T("entrar") }
func (p *pantEntrar) SeccionActiva() string { return "" }

func (p *pantEntrar) Recargar(a *App) tea.Cmd {
	t := a.tema
	base := lipgloss.NewStyle().Background(t.Fondo).Foreground(t.Texto)
	s := p.input.Styles()
	for _, st := range []*textinput.StyleState{&s.Focused, &s.Blurred} {
		st.Text, st.Prompt, st.Suggestion = base, base, base
		st.Placeholder = base.Foreground(t.Tenue)
	}
	s.Cursor.Color = t.Fosforo
	p.input.SetStyles(s)
	return nil
}

func (p *pantEntrar) Teclas(a *App) []Atajo {
	if p.ocupado {
		return []Atajo{{"Esc", T("cancelar")}}
	}
	return []Atajo{{"↑↓", T("k_elegir")}, {"Enter", T("k_abrir")}, {"Esc", T("k_volver")}}
}

func (p *pantEntrar) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	s := k.String()
	if p.ocupado {
		if s == "esc" && p.cancelar != nil {
			p.cancelar()
		}
		return true, nil
	}
	if p.pegando {
		switch s {
		case "esc":
			p.pegando = false
			p.input.Blur()
			return true, nil
		case "enter":
			v := strings.TrimSpace(p.input.Value())
			v = strings.TrimPrefix(v, "sid=")
			if v == "" {
				return true, nil
			}
			return true, p.verificar(a, map[string]string{"sid": v})
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(k)
		return true, cmd
	}
	switch s {
	case "up", "down", "tab", "shift+tab", "k", "j":
		p.foco = 1 - p.foco
	case "enter", "space":
		if p.foco == 0 {
			return true, p.conGoogle(a)
		}
		p.pegando = true
		return true, p.input.Focus()
	default:
		return false, nil
	}
	return true, nil
}

func (p *pantEntrar) conGoogle(a *App) tea.Cmd {
	nav, err := entrar.Buscar()
	if err != nil {
		p.err = T("entrar_sin_nav")
		return nil
	}
	p.err = ""
	p.ocupado = true
	p.estado = T("entrar_como", nav.Nombre) + " " + T("entrar_esperando")
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	p.cancelar = cancel
	return func() tea.Msg {
		defer cancel()
		r, err := entrar.Entrar(ctx, site.BaseURL, config.DirPerfil(), func(string) {})
		if err != nil {
			return entrarMsg{p: p, err: err}
		}
		return entrarMsg{p: p, cookies: r.Cookies}
	}
}

// verificar prueba las cookies nuevas pidiendo una página: si el sitio no ve la sesión, no sirven.
func (p *pantEntrar) verificar(a *App, nuevas map[string]string) tea.Cmd {
	p.ocupado = true
	p.err = ""
	antes := a.cliente.Cookies()
	todas := map[string]string{}
	for k, v := range antes {
		todas[k] = v
	}
	for k, v := range nuevas {
		todas[k] = v
	}
	a.cliente.SetCookies(todas)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		cab, err := a.cliente.Estado(ctx)
		return verificadoMsg{p, cab, err, antes}
	}
}

func (p *pantEntrar) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case entrarMsg:
		if m.p != p {
			return nil
		}
		p.cancelar = nil
		if m.err != nil {
			p.ocupado = false
			p.estado = ""
			switch {
			case errors.Is(m.err, entrar.ErrCerrado):
				p.err = T("entrar_cerrado")
			case errors.Is(m.err, context.Canceled):
				p.err = ""
			default:
				p.err = m.err.Error()
			}
			return nil
		}
		return p.verificar(a, m.cookies)
	case verificadoMsg:
		if m.p != p {
			return nil
		}
		p.ocupado = false
		if m.err != nil || !m.cab.Conectado {
			a.cliente.SetCookies(m.antes)
			if m.err != nil {
				p.err = textoError(m.err)
			} else {
				p.err = T("entrar_invalida")
			}
			return nil
		}
		a.actualizarCabecera(m.cab)
		a.avisar(T("entrar_ok"))
		p.listo = true
		// Volver a donde estaba, recargado (ahora con la sesión).
		if len(a.pila) > 1 {
			a.Volver()
			return a.actual().Recargar(a)
		}
		return a.Reemplazar(nuevaListado("", false, a.prefs.Vista))
	case tea.PasteMsg:
		if p.pegando {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(m)
			return cmd
		}
	default:
		if p.pegando {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return cmd
		}
	}
	return nil
}

func (p *pantEntrar) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	es := a.es
	ls := a.h1(T("entrar"), w)
	ls = append(ls, a.parrafo(T("entrar_texto"), w, es.texto)...)
	ls = append(ls, nil)
	y := len(ls)
	b1 := a.boton(T("entrar_google"), p.foco == 0 && !p.pegando)
	ls = append(ls, b1)
	z.agregar(0, y, b1.Ancho(), y+1, func() tea.Cmd { p.foco = 0; return p.conGoogle(a) })
	ls = append(ls, nil)
	y = len(ls)
	b2 := a.secundario(T("entrar_pegar"), p.foco == 1 && !p.pegando)
	ls = append(ls, b2)
	z.agregar(0, y, b2.Ancho(), y+1, func() tea.Cmd { p.foco, p.pegando = 1, true; return p.input.Focus() })
	if p.pegando {
		ls = append(ls, a.parrafo(T("entrar_pegar_ayuda"), w, es.tenue)...)
		p.input.SetWidth(max(10, w-4))
		ls = append(ls, caja([]Linea{mas(Linea{espacio(es.fondo, 1)}, Linea{crudo(p.input.View())})}, w, bordeSimple, es.acento, es.fondo)...)
	}
	ls = append(ls, nil)
	if p.estado != "" && p.ocupado {
		ls = append(ls, a.parrafo(p.estado, w, es.aviso)...)
		ls = append(ls, a.parrafo(T("entrar_google_bloqueo"), w, es.tenue)...)
	}
	if p.err != "" {
		ls = append(ls, a.parrafo(T("error")+p.err, w, es.error.negrita())...)
	}
	ls = append(ls, nil, txt(es.tenue, recortar(site.BaseURL+"/privacidad", w)))
	return rellenar(ls, alto)
}

// ─── Preferencias (lo que en el sitio está en Mi cuenta) ────────────────────────────────────

type pantPreferencias struct {
	fila, col    int
	actualizando bool
}

type actualizadoMsg struct {
	version string
	err     error
}
type salidoMsg struct{ err error }

func nuevaPreferencias() *pantPreferencias { return &pantPreferencias{} }

func (p *pantPreferencias) Titulo() string          { return T("preferencias") }
func (p *pantPreferencias) SeccionActiva() string   { return "" }
func (p *pantPreferencias) Recargar(a *App) tea.Cmd { return nil }

func (p *pantPreferencias) filas(a *App) [][]string {
	temas := []string{}
	for _, t := range Temas {
		temas = append(temas, t.Nombre)
	}
	cuenta := []string{T("entrar")}
	if a.conectado() {
		cuenta = []string{T("salir")}
	}
	f := [][]string{temas, {T("catalogo"), T("lista")}, {T("idioma_sistema"), "español", "English"}, cuenta}
	if a.op.Actualizar != nil {
		f = append(f, []string{T("actualizar")})
	}
	return f
}

func (p *pantPreferencias) Teclas(a *App) []Atajo {
	return []Atajo{{"↑↓", T("k_mover")}, {"←→", T("k_elegir")}, {"Enter", T("k_elegir")}, {"Esc", T("k_volver")}}
}

func (p *pantPreferencias) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	fs := p.filas(a)
	switch k.String() {
	case "up", "k":
		p.fila = (p.fila - 1 + len(fs)) % len(fs)
		p.col = 0
	case "down", "j", "tab":
		p.fila = (p.fila + 1) % len(fs)
		p.col = 0
	case "left", "h":
		p.col = max(0, p.col-1)
	case "right", "l":
		p.col = min(len(fs[p.fila])-1, p.col+1)
	case "enter", "space":
		return true, p.elegir(a, p.fila, p.col)
	default:
		return false, nil
	}
	return true, nil
}

func (p *pantPreferencias) elegir(a *App, fila, col int) tea.Cmd {
	p.fila, p.col = fila, col
	switch fila {
	case 0:
		a.usarTema(Temas[col].Nombre)
	case 1:
		a.prefs.Vista = []string{"catalogo", "lista"}[col]
		for _, pa := range a.pila {
			if l, ok := pa.(*pantListado); ok && l.vista != a.prefs.Vista {
				l.vista = a.prefs.Vista
				l.fichas = nil
				l.sitioPag = 0
			}
		}
	case 2:
		a.prefs.Idioma = []string{"", "es", "en"}[col]
		Usar(a.prefs.Idioma)
	case 3:
		if a.conectado() {
			a.capa = nuevoConfirmar(T("confirmar_salir"), T("salir"), func() tea.Cmd { return a.salirDeCuenta() })
			return nil
		}
		return a.Ir(nuevaEntrar())
	case 4:
		return p.actualizar(a)
	}
	config.GuardarPreferencias(a.prefs)
	return nil
}

func (p *pantPreferencias) actualizar(a *App) tea.Cmd {
	if a.op.Actualizar == nil || p.actualizando {
		return nil
	}
	p.actualizando = true
	a.avisar(T("actualizando"))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
		defer cancel()
		v, err := a.op.Actualizar(ctx)
		return actualizadoMsg{v, err}
	}
}

func (p *pantPreferencias) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	if m, ok := msg.(actualizadoMsg); ok {
		p.actualizando = false
		switch {
		case m.err != nil:
			a.fallar(m.err)
		case m.version == a.op.Version:
			a.avisar(T("al_dia", m.version))
		default:
			a.nueva = ""
			a.avisar(T("actualizado", m.version))
		}
	}
	return nil
}

// salirDeCuenta cierra la sesión en el sitio y la olvida.
func (a *App) salirDeCuenta() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		err := a.cliente.Salir(ctx)
		return salidoMsg{err}
	}
}

func (p *pantPreferencias) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	es := a.es
	ls := a.h1(T("preferencias"), w)
	nombres := []string{T("tema"), T("listados"), T("idioma"), T("cuenta"), mayus(T("version", a.op.Version))}
	actual := func(fila, col int) bool {
		switch fila {
		case 0:
			return Temas[col].Nombre == a.tema.Nombre
		case 1:
			return []string{"catalogo", "lista"}[col] == a.prefs.Vista
		case 2:
			return []string{"", "es", "en"}[col] == a.prefs.Idioma
		}
		return false
	}
	for i, opciones := range p.filas(a) {
		ls = append(ls, nil)
		e := es.tenue
		if i == p.fila {
			e = es.acento
		}
		ls = append(ls, txt(e, mayus(nombres[i])))
		if i == 3 {
			estado := T("desconectado")
			if a.conectado() {
				estado = T("conectado")
			}
			ls = append(ls, txt(es.texto, recortar(estado, w)))
		}
		y := len(ls)
		var l Linea
		x := 0
		for j, o := range opciones {
			if j > 0 {
				l = append(l, espacio(es.fondo, 1))
				x++
			}
			var pieza Linea
			if i == 0 {
				// Muestra "Aa" con los colores de cada tema, como en Mi cuenta.
				tt := Temas[j]
				pieza = Linea{{Est{Fg: tt.Fosforo, Bg: tt.Fondo, Negrita: true}, " Aa "}, {es.fondo, " "}}
			}
			sel := i == p.fila && j == p.col
			switch {
			case actual(i, j) && sel:
				pieza = append(pieza, Run{es.invertido, "[" + o + " ✓]"})
			case actual(i, j):
				pieza = append(pieza, Run{es.invertido, " " + o + " ✓ "})
			case sel:
				pieza = append(pieza, Run{es.acento.subrayado(), "[" + o + "]"})
			default:
				pieza = append(pieza, Run{es.acento, " " + o + " "})
			}
			fila, col := i, j
			z.agregar(x, y, x+pieza.Ancho(), y+1, func() tea.Cmd { return p.elegir(a, fila, col) })
			l = mas(l, pieza)
			x += pieza.Ancho()
		}
		ls = append(ls, envolverLinea(l, w)...)
	}
	return rellenar(ls, alto)
}
