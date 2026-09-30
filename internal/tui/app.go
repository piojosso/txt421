// Package tui es la interfaz de pantalla completa de txt421: imita a txt.421.news (colores,
// fichas, mensajes, barras) y reemplaza el scroll por páginas del tamaño de la ventana.
package tui

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/piojosso/txt421/internal/config"
	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// Tamaño mínimo de la ventana.
const (
	minAncho = 60
	minAlto  = 18
	// Ancho máximo del contenido: el <main> del sitio mide 52rem.
	maxContenido = 96
)

// Pantalla es una vista de la app (portada, publicación, búsqueda…).
type Pantalla interface {
	// Dibujar devuelve exactamente alto líneas de ancho columnas y registra sus zonas de mouse.
	Dibujar(a *App, z *Zonas, ancho, alto int) []Linea
	// Tecla atiende una tecla; devuelve si la usó.
	Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd)
	// Mensaje atiende resultados de pedidos y otros mensajes.
	Mensaje(a *App, msg tea.Msg) tea.Cmd
	// Teclas para la barra de abajo.
	Teclas(a *App) []Atajo
	// Titulo de la ventana.
	Titulo() string
	// Recargar vuelve a pedir lo que muestra.
	Recargar(a *App) tea.Cmd
	// Seccion activa (para marcarla en la barra de arriba): slug, "" = ninguna.
	SeccionActiva() string
}

// Atajo: una tecla y lo que hace, para la barra de abajo.
type Atajo struct{ Tecla, Que string }

// Opciones con las que arranca la app.
type Opciones struct {
	Version   string
	UserAgent string
	// Qué abrir primero: "", "h/123", "b/juegos", "buscar/texto", "respuestas", "entrar".
	Abrir string
	// Actualizar busca y descarga una versión nueva. Nil = no hay actualizador.
	Actualizar func(ctx context.Context) (version string, err error)
	// UltimaVersion consulta la última versión publicada.
	UltimaVersion func(ctx context.Context) (string, error)
}

// App es el modelo de Bubble Tea.
type App struct {
	op      Opciones
	cliente *site.Cliente
	prefs   config.Preferencias
	tema    Tema
	es      paleta
	cab     site.Cabecera // lo último que se supo de la sesión
	sabeCab bool

	pila       []Pantalla
	capa       Capa // menú, ayuda o diálogo encima de todo
	ancho      int
	alto       int
	zonas      Zonas
	nota       nota // mensaje abajo (aviso o error), dura unos segundos
	cursor     bool // el ▮ del logo titila
	ctx        context.Context
	cancel     context.CancelFunc
	nueva      string // versión nueva disponible
	borradores map[string]config.Borrador
}

// Capa es algo que se dibuja encima de la pantalla y recibe las teclas primero.
type Capa interface {
	Dibujar(a *App, z *Zonas, ancho, alto int) (lineas []Linea, x, y int)
	Tecla(a *App, k tea.KeyPressMsg) tea.Cmd
	Mensaje(a *App, msg tea.Msg) tea.Cmd
}

type nota struct {
	texto string
	error bool
	hasta time.Time
}

// Nueva crea la app.
func Nueva(op Opciones) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{op: op, ctx: ctx, cancel: cancel}
	a.prefs = config.CargarPreferencias()
	Usar(a.prefs.Idioma)
	a.usarTema(a.prefs.Tema)
	a.cliente = site.Nuevo(op.UserAgent)
	a.cliente.SetCookies(config.CargarCookies())
	a.borradores = config.CargarBorradores()
	return a
}

func (a *App) usarTema(n string) {
	a.tema = TemaPorNombre(n)
	a.es = nuevaPaleta(a.tema)
	a.prefs.Tema = a.tema.Nombre
}

// ─── Bubble Tea ─────────────────────────────────────────────────────────────────────────────

type tickCursor struct{}
type tickNota struct{}

func (a *App) Init() tea.Cmd {
	var inicial Pantalla
	abrir := a.op.Abrir
	switch {
	case strings.HasPrefix(abrir, "h/"):
		no, post := parseRefHilo(strings.TrimPrefix(abrir, "h/"))
		inicial = nuevaHilo(no, post)
	case strings.HasPrefix(abrir, "p/"):
		inicial = nuevaHiloPorPost(atoi(strings.TrimPrefix(abrir, "p/")))
	case strings.HasPrefix(abrir, "b/"):
		inicial = nuevaListado(strings.TrimPrefix(abrir, "b/"), false, a.prefs.Vista)
	case strings.HasPrefix(abrir, "buscar/"):
		inicial = nuevaBuscar(strings.TrimPrefix(abrir, "buscar/"))
	case abrir == "respuestas":
		inicial = nuevaBandeja()
	case abrir == "guardados":
		inicial = nuevaGuardados()
	case abrir == "entrar":
		inicial = nuevaEntrar()
	default:
		inicial = nuevaListado("", false, a.prefs.Vista)
	}
	a.pila = []Pantalla{inicial}
	cmds := []tea.Cmd{inicial.Recargar(a), tickCursorCmd(), a.chequearVersion()}
	// Si se abrió directo algo que no es la portada, "volver" lleva a la portada.
	if abrir != "" {
		a.pila = []Pantalla{nuevaListado("", false, a.prefs.Vista), inicial}
	}
	return tea.Batch(cmds...)
}

func tickCursorCmd() tea.Cmd {
	return tea.Tick(1100*time.Millisecond/2, func(time.Time) tea.Msg { return tickCursor{} })
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.ancho, a.alto = m.Width, m.Height
		return a, nil
	case tickCursor:
		a.cursor = !a.cursor
		return a, tickCursorCmd()
	case tickNota:
		return a, nil
	case versionMsg:
		a.prefs.UltimoChequeo = time.Now()
		config.GuardarPreferencias(a.prefs)
		if m.version != "" && m.version != a.op.Version {
			a.nueva = m.version
			a.avisar(T("hay_version", m.version))
		}
		return a, nil
	case salidoMsg:
		config.GuardarCookies(a.cliente.Cookies())
		a.cab = site.Cabecera{}
		a.sabeCab = true
		if m.err != nil {
			a.fallar(m.err)
		} else {
			a.avisar(T("saliste"))
		}
		return a, a.actual().Recargar(a)
	case tea.KeyPressMsg:
		return a, a.tecla(m)
	case tea.MouseClickMsg:
		return a, a.click(m.Mouse())
	case tea.MouseWheelMsg:
		return a, a.rueda(m.Mouse())
	case tea.PasteMsg:
		if a.capa != nil {
			return a, a.capa.Mensaje(a, m)
		}
		return a, a.actual().Mensaje(a, m)
	}
	// Resultados de pedidos y demás: a la capa y a todas las pantallas (cada una reconoce lo suyo).
	var cmds []tea.Cmd
	if a.capa != nil {
		cmds = append(cmds, a.capa.Mensaje(a, msg))
	}
	for _, p := range a.pila {
		cmds = append(cmds, p.Mensaje(a, msg))
	}
	return a, tea.Batch(cmds...)
}

func (a *App) actual() Pantalla { return a.pila[len(a.pila)-1] }

// Ir abre una pantalla nueva encima de la actual.
func (a *App) Ir(p Pantalla) tea.Cmd {
	a.capa = nil
	a.pila = append(a.pila, p)
	if len(a.pila) > 30 {
		a.pila = append(a.pila[:1], a.pila[len(a.pila)-29:]...)
	}
	return p.Recargar(a)
}

// Reemplazar cambia la pantalla actual por otra.
func (a *App) Reemplazar(p Pantalla) tea.Cmd {
	a.capa = nil
	a.pila[len(a.pila)-1] = p
	return p.Recargar(a)
}

// Volver cierra la pantalla actual. En la primera no hace nada.
func (a *App) Volver() tea.Cmd {
	if len(a.pila) > 1 {
		a.pila = a.pila[:len(a.pila)-1]
		if r, ok := a.actual().(interface{ AlVolver(*App) tea.Cmd }); ok {
			return r.AlVolver(a)
		}
	}
	return nil
}

// Inicio vuelve a la portada, cerrando todo lo demás.
func (a *App) Inicio() tea.Cmd {
	if l, ok := a.pila[0].(*pantListado); ok && l.seccion == "" && !l.archivo {
		a.pila = a.pila[:1]
		a.capa = nil
		return l.Recargar(a)
	}
	a.pila = a.pila[:0]
	return a.Ir(nuevaListado("", false, a.prefs.Vista))
}

// abrirSeccion va a una sección ("" = portada) desde cualquier lado.
func (a *App) abrirSeccion(slug string) tea.Cmd {
	if slug == "" {
		return a.Inicio()
	}
	if l, ok := a.actual().(*pantListado); ok {
		return a.Reemplazar(nuevaListado(slug, false, l.vista))
	}
	return a.Ir(nuevaListado(slug, false, a.prefs.Vista))
}

// Salir de la app.
func (a *App) Salir() tea.Cmd {
	a.cancel()
	a.guardarBorradores()
	return tea.Quit
}

func (a *App) avisar(s string) {
	a.nota = nota{texto: s, hasta: time.Now().Add(6 * time.Second)}
}

func (a *App) fallar(err error) {
	if err == nil {
		return
	}
	a.nota = nota{texto: textoError(err), error: true, hasta: time.Now().Add(8 * time.Second)}
}

// textoError traduce errores del cliente a algo para mostrar.
func textoError(err error) string {
	var es *site.ErrSitio
	var er *site.ErrRed
	switch {
	case errors.Is(err, site.ErrSesion):
		return T("necesita_entrar")
	case errors.Is(err, site.ErrNoEncontrado):
		return T("no_existe")
	case errors.As(err, &es):
		return es.Mensaje
	case errors.As(err, &er):
		return T("sin_red")
	case errors.Is(err, context.Canceled):
		return ""
	}
	return err.Error()
}

// actualizarCabecera guarda lo que dijo el sitio de la sesión, y la persiste si cambió.
func (a *App) actualizarCabecera(c site.Cabecera) {
	a.cab = c
	a.sabeCab = true
	// La cookie de visita cambia en cada página: se guarda siempre.
	config.GuardarCookies(a.cliente.Cookies())
}

func (a *App) conectado() bool { return a.cab.Conectado || (!a.sabeCab && a.cliente.Sesion() != "") }

// ─── Teclado y mouse ────────────────────────────────────────────────────────────────────────

func (a *App) tecla(k tea.KeyPressMsg) tea.Cmd {
	if k.String() == "ctrl+c" {
		return a.Salir()
	}
	if a.capa != nil {
		return a.capa.Tecla(a, k)
	}
	if a.chica() {
		if k.String() == "q" {
			return a.Salir()
		}
		return nil
	}
	if usada, cmd := a.actual().Tecla(a, k); usada {
		return cmd
	}
	// Teclas generales (si la pantalla no las usó).
	switch k.String() {
	case "q":
		return a.Salir()
	case "esc", "backspace":
		return a.Volver()
	case "?", "f1":
		a.capa = &capaAyuda{}
	case "m", "f10", "alt+m":
		a.capa = nuevoMenu(a)
	case "/":
		return a.Ir(nuevaBuscar(""))
	case "e":
		if a.conectado() {
			return a.Ir(nuevaBandeja())
		}
		return a.Ir(nuevaEntrar())
	case "t":
		a.siguienteTema()
	case "0", "h":
		return a.Inicio()
	case "1", "2", "3", "4", "5":
		return a.abrirSeccion(site.Secciones[int(k.String()[0]-'1')].Slug)
	case "tab", "shift+tab":
		return a.cambiarSeccion(k.String() == "tab")
	case "r", "f5":
		return a.actual().Recargar(a)
	}
	return nil
}

// cambiarSeccion: Tab recorre Portada → Tecnología → … → Vida real → Portada.
func (a *App) cambiarSeccion(adelante bool) tea.Cmd {
	orden := []string{""}
	for _, s := range site.Secciones {
		orden = append(orden, s.Slug)
	}
	act := a.actual().SeccionActiva()
	i := 0
	for j, s := range orden {
		if s == act {
			i = j
		}
	}
	if adelante {
		i = (i + 1) % len(orden)
	} else {
		i = (i - 1 + len(orden)) % len(orden)
	}
	return a.abrirSeccion(orden[i])
}

func (a *App) siguienteTema() {
	for i, t := range Temas {
		if t.Nombre == a.tema.Nombre {
			a.usarTema(Temas[(i+1)%len(Temas)].Nombre)
			break
		}
	}
	config.GuardarPreferencias(a.prefs)
	a.avisar(T("tema") + ": " + a.tema.Nombre)
}

func (a *App) click(m tea.Mouse) tea.Cmd {
	if m.Button != tea.MouseLeft {
		return nil
	}
	if z := a.zonas.en(m.X, m.Y); z != nil && z.accion != nil {
		return z.accion()
	}
	// Click afuera de una capa: la cierra.
	if a.capa != nil {
		a.capa = nil
	}
	return nil
}

func (a *App) rueda(m tea.Mouse) tea.Cmd {
	// Sin scroll: la rueda mueve la selección, como las flechas.
	k := tea.KeyPressMsg{Code: tea.KeyDown}
	if m.Button == tea.MouseWheelUp {
		k = tea.KeyPressMsg{Code: tea.KeyUp}
	}
	return a.tecla(k)
}

// ─── Dibujo ─────────────────────────────────────────────────────────────────────────────────

func (a *App) chica() bool { return a.ancho < minAncho || a.alto < minAlto }

func (a *App) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = a.tema.Fondo
	if a.ancho == 0 {
		return v
	}
	a.zonas = Zonas{}
	fondo := a.es.fondo
	if a.chica() {
		var ls []string
		for i := 0; i < a.alto; i++ {
			l := Linea{}
			if i == a.alto/2 {
				l = centrar(txt(a.es.tenue, T("chica", minAncho, minAlto)), a.ancho, fondo)
			}
			ls = append(ls, l.Render(a.ancho, fondo))
		}
		v.SetContent(strings.Join(ls, "\n"))
		return v
	}
	p := a.actual()
	v.WindowTitle = "txt · " + p.Titulo()

	cab := a.cabecera(p)
	pie := a.pie(p)
	altoCont := a.alto - len(cab) - len(pie)
	anchoCont := min(a.ancho-2, maxContenido)
	x0 := (a.ancho - anchoCont) / 2
	y0 := len(cab)

	zc := &Zonas{}
	cont := p.Dibujar(a, zc, anchoCont, altoCont)
	a.zonas.sumar(zc, x0, y0)

	pantalla := make([]Linea, 0, a.alto)
	pantalla = append(pantalla, cab...)
	for i := 0; i < altoCont; i++ {
		var l Linea
		if i < len(cont) {
			l = cont[i]
		}
		pantalla = append(pantalla, mas(Linea{espacio(fondo, x0)}, completar(l, anchoCont, fondo)))
	}
	pantalla = append(pantalla, pie...)

	if a.capa != nil {
		zc := &Zonas{}
		ls, x, y := a.capa.Dibujar(a, zc, a.ancho, a.alto)
		// Las zonas de la capa van primero: tapan a las de abajo.
		base := a.zonas
		a.zonas = Zonas{}
		a.zonas.sumar(zc, x, y)
		a.zonas.lista = append(a.zonas.lista, base.lista...)
		for i, l := range ls {
			if y+i >= 0 && y+i < len(pantalla) {
				pantalla[y+i] = superponer(pantalla[y+i], l, x, fondo)
			}
		}
	}

	out := make([]string, len(pantalla))
	for i, l := range pantalla {
		out[i] = l.Render(a.ancho, fondo)
	}
	v.SetContent(strings.Join(out, "\n"))
	return v
}

// superponer pone arriba en la columna x de abajo.
func superponer(abajo, arriba Linea, x int, fondo Est) Linea {
	izq := completar(cortarLinea(abajo, 0, x), x, fondo)
	w := arriba.Ancho()
	der := cortarLinea(abajo, x+w, 1<<20)
	return mas(izq, arriba, der)
}

// cortarLinea devuelve las columnas [desde, hasta) de una línea.
func cortarLinea(l Linea, desde, hasta int) Linea {
	var out Linea
	col := 0
	for _, r := range l {
		if r.E.Crudo {
			w := ancho(r.T)
			ini, fin := max(desde, col), min(hasta, col+w)
			if ini < fin {
				out = append(out, crudo(ansi.Cut(r.T, ini-col, fin-col)))
			}
			col += w
			continue
		}
		for _, c := range r.T {
			w := ancho(string(c))
			if col >= desde && col+w <= hasta {
				if len(out) > 0 && out[len(out)-1].E == r.E {
					out[len(out)-1].T += string(c)
				} else {
					out = append(out, Run{r.E, string(c)})
				}
			}
			col += w
		}
	}
	return out
}

// cabecera: ">_ TXT▮" y los íconos, las secciones y la línea de fósforo (.cabecera del sitio).
func (a *App) cabecera(p Pantalla) []Linea {
	es := a.es
	W := a.ancho
	anchoCont := min(W-2, maxContenido)
	x0 := (W - anchoCont) / 2
	fondo := es.fondo

	cursor := "▮"
	if !a.cursor {
		cursor = " "
	}
	logo := Linea{{es.acento.negrita(), ">_ TXT"}, {es.acento, cursor}}
	a.zonas.agregar(x0, 0, x0+logo.Ancho(), 1, func() tea.Cmd { return a.Inicio() })

	// A la derecha, como los íconos del sitio pero con palabras (los símbolos de lupa, sol y sobre
	// faltan en muchas fuentes de terminal): [BUSCAR] [TEMA] [RESPUESTAS 3] [MENÚ] o [ENTRAR].
	type icono struct {
		texto  string
		est    Est
		badge  string
		accion func() tea.Cmd
	}
	iconos := []icono{
		{mayus(T("buscar")), es.acento, "", func() tea.Cmd { return a.Ir(nuevaBuscar("")) }},
		{mayus(T("tema")), es.acento, "", func() tea.Cmd { a.siguienteTema(); return nil }},
	}
	if a.conectado() {
		ic := icono{mayus(T("respuestas")), es.acento, "", func() tea.Cmd { return a.Ir(nuevaBandeja()) }}
		if a.cab.Novedades > 0 {
			// .correo.hay-novedades: en color de aviso, con el número en una etiqueta.
			ic.est = es.aviso.negrita()
			ic.badge = " " + itoa(a.cab.Novedades) + " "
		}
		iconos = append(iconos, ic)
	} else {
		iconos = append(iconos, icono{mayus(T("entrar")), es.acento, "", func() tea.Cmd { return a.Ir(nuevaEntrar()) }})
	}
	iconos = append(iconos, icono{mayus(T("menu")), es.acento, "", func() tea.Cmd { a.capa = nuevoMenu(a); return nil }})
	// Si no entra, primero se va "Tema" (sigue en t y en Preferencias).
	medir := func(ics []icono) int {
		n := 0
		for _, ic := range ics {
			n += ancho(ic.texto) + ancho(ic.badge) + 3
		}
		return n
	}
	if medir(iconos)+logo.Ancho()+2 > anchoCont {
		iconos = append(iconos[:1], iconos[2:]...)
	}
	var der Linea
	for i, ic := range iconos {
		if i > 0 {
			der = append(der, espacio(fondo, 1))
		}
		der = append(der, Run{es.tenue, "["}, Run{ic.est, ic.texto})
		if ic.badge != "" {
			der = append(der, Run{Est{Fg: a.tema.Fondo, Bg: a.tema.Aviso, Negrita: true}, ic.badge})
		}
		der = append(der, Run{es.tenue, "]"})
	}
	xDer := x0 + anchoCont - der.Ancho()
	col := xDer
	for i, ic := range iconos {
		if i > 0 {
			col++
		}
		w := ancho(ic.texto) + ancho(ic.badge) + 2
		accion := ic.accion
		a.zonas.agregar(col, 0, col+w, 1, accion)
		col += w
	}
	fila1 := mas(Linea{espacio(fondo, x0)}, aDerecha(logo, der, anchoCont, fondo))

	// Secciones.
	activa := p.SeccionActiva()
	var secs Linea
	col = x0
	for i, s := range site.Secciones {
		if i > 0 {
			secs = append(secs, espacio(fondo, 2))
			col += 2
		}
		e := es.tenue
		if s.Slug == activa {
			e = es.acento.subrayado()
		}
		nombre := mayus(s.Nombre)
		secs = append(secs, Run{e, nombre})
		slug := s.Slug
		a.zonas.agregar(col, 1, col+ancho(nombre), 2, func() tea.Cmd { return a.abrirSeccion(slug) })
		col += ancho(nombre)
	}
	if secs.Ancho()+ancho(T("leer421"))+5 <= anchoCont {
		secs = append(secs, espacio(fondo, 2), Run{es.borde, "|"}, espacio(fondo, 2))
		col += 5
		l421 := mayus(T("leer421"))
		secs = append(secs, Run{es.acento, l421})
		a.zonas.agregar(col, 1, col+ancho(l421), 2, func() tea.Cmd {
			abrirNavegador("https://www.421.news/es/?utm_source=txt&utm_medium=secciones")
			return nil
		})
	}
	fila2 := mas(Linea{espacio(fondo, x0)}, secs)
	linea := txt(es.acento, strings.Repeat("─", W))
	return []Linea{fila1, fila2, linea}
}

// pie: la línea de fósforo y las teclas (como la barra de abajo del sitio en el celular).
func (a *App) pie(p Pantalla) []Linea {
	es := a.es
	W := a.ancho
	fondo := es.fondo
	var fila Linea
	if a.nota.texto != "" && time.Now().Before(a.nota.hasta) {
		e := es.aviso
		pre := "[!] "
		if a.nota.error {
			e = es.error.negrita()
			pre = T("error")
		}
		fila = txt(e, " "+pre+a.nota.texto)
	} else {
		var ats []Atajo
		if a.capa != nil {
			if c, ok := a.capa.(interface{ Teclas() []Atajo }); ok {
				ats = c.Teclas()
			}
		} else {
			ats = p.Teclas(a)
		}
		fila = Linea{espacio(fondo, 1)}
		for i, at := range ats {
			bloque := Linea{{es.acento.negrita(), at.Tecla}, {es.tenue, " " + at.Que}}
			sep := Linea{{es.borde, " · "}}
			if i > 0 {
				if fila.Ancho()+sep.Ancho()+bloque.Ancho() > W-1 {
					break
				}
				fila = mas(fila, sep)
			}
			fila = mas(fila, bloque)
		}
	}
	return []Linea{txt(es.acento, strings.Repeat("─", W)), completar(fila, W, fondo)}
}

// ─── Estilos ────────────────────────────────────────────────────────────────────────────────

// paleta: los estilos de uso común, ya con el fondo puesto.
type paleta struct {
	fondo, texto, tenue, acento, borde, cita, aviso, error Est
	// Sobre la caja (fichas y mensajes).
	cajaFondo, cajaTexto, cajaTenue, cajaAcento, cajaBorde, cajaCita, cajaAviso Est
	// Invertido: fondo de acento (botón activo, OP, página actual).
	invertido Est
	sel       color.Color // fondo del mensaje seleccionado (:target)
	fijada    color.Color // fondo de una ficha fijada
}

func nuevaPaleta(t Tema) paleta {
	f := Est{Bg: t.Fondo}
	c := Est{Bg: t.Caja}
	return paleta{
		fondo: f.conFg(t.Texto), texto: f.conFg(t.Texto), tenue: f.conFg(t.Tenue), acento: f.conFg(t.Fosforo),
		borde: f.conFg(t.Borde), cita: f.conFg(t.Cita), aviso: f.conFg(t.Aviso), error: f.conFg(t.Error),
		cajaFondo: c.conFg(t.Texto), cajaTexto: c.conFg(t.Texto), cajaTenue: c.conFg(t.Tenue), cajaAcento: c.conFg(t.Fosforo),
		cajaBorde: c.conFg(t.Borde), cajaCita: c.conFg(t.Cita), cajaAviso: c.conFg(t.Aviso),
		invertido: Est{Fg: t.Fondo, Bg: t.Fosforo, Negrita: true},
		sel:       mezclar(t.Fosforo, t.Caja, 0.10),
		fijada:    mezclar(t.Aviso, t.Caja, 0.07),
	}
}

// textoSobre: estilos para el texto de un mensaje sobre un fondo dado.
func (a *App) textoSobre(bg color.Color) estilosTexto {
	t := a.tema
	b := Est{Bg: bg}
	return estilosTexto{
		normal:       b.conFg(t.Texto),
		cita:         b.conFg(t.Cita),
		ref:          b.conFg(t.Cita).subrayado(),
		spoiler:      Est{Fg: t.Fosforo, Bg: t.Fosforo},
		spoilerVisto: b.conFg(t.Texto).subrayado(),
		marca:        Est{Fg: t.Fondo, Bg: t.Fosforo},
	}
}

// ─── Zonas de mouse ─────────────────────────────────────────────────────────────────────────

type zona struct {
	x0, y0, x1, y1 int
	accion         func() tea.Cmd
}

// Zonas: rectángulos clickeables del último cuadro.
type Zonas struct{ lista []zona }

func (z *Zonas) agregar(x0, y0, x1, y1 int, accion func() tea.Cmd) {
	z.lista = append(z.lista, zona{x0, y0, x1, y1, accion})
}

func (z *Zonas) sumar(otra *Zonas, dx, dy int) {
	for _, o := range otra.lista {
		z.lista = append(z.lista, zona{o.x0 + dx, o.y0 + dy, o.x1 + dx, o.y1 + dy, o.accion})
	}
}

func (z *Zonas) en(x, y int) *zona {
	for i := range z.lista {
		o := &z.lista[i]
		if x >= o.x0 && x < o.x1 && y >= o.y0 && y < o.y1 {
			return o
		}
	}
	return nil
}

// ─── Borradores ─────────────────────────────────────────────────────────────────────────────

func (a *App) guardarBorradores() {
	config.GuardarBorradores(a.borradores)
}

// ─── Versión ────────────────────────────────────────────────────────────────────────────────

type versionMsg struct{ version string }

func (a *App) chequearVersion() tea.Cmd {
	if a.op.UltimaVersion == nil || a.op.Version == "dev" {
		return nil
	}
	if time.Since(a.prefs.UltimoChequeo) < 20*time.Hour && a.prefs.AvisoVersion == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
		defer cancel()
		v, err := a.op.UltimaVersion(ctx)
		if err != nil {
			return nil
		}
		return versionMsg{v}
	}
}
