package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/piojosso/txt421/internal/config"
	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// pantListado: portada, sección o archivo. Junta las páginas del sitio (60 fichas en catálogo,
// 10 en lista) y las reparte en páginas del tamaño de la ventana.
type pantListado struct {
	seccion string // slug; "" = portada
	archivo bool
	vista   string

	fichas      []site.Ficha
	sitioPag    int // última página del sitio cargada
	sitioTotal  int
	titulo      string
	descripcion string
	cargando    bool
	err         error
	pedido      int // para descartar respuestas viejas

	sel       int
	selHilo   int // al recargar, volver a elegir esta publicación (el orden cambia)
	pagActual int
	// Lo que midió el último dibujo (para las teclas).
	cols, porPagina int
	paginas         []rango // vista lista
}

type listadoMsg struct {
	p      *pantListado
	pedido int
	pagina int
	l      *site.Listado
	err    error
}

func nuevaListado(seccion string, archivo bool, vista string) *pantListado {
	if vista != "lista" {
		vista = "catalogo"
	}
	return &pantListado{seccion: seccion, archivo: archivo, vista: vista}
}

func (p *pantListado) Titulo() string {
	if p.seccion == "" {
		return T("portada")
	}
	s := site.SeccionPorSlug(p.seccion)
	if s == nil {
		return p.seccion
	}
	if p.archivo {
		return s.Nombre + " · " + T("archivo")
	}
	return s.Nombre
}

func (p *pantListado) SeccionActiva() string { return p.seccion }

func (p *pantListado) Recargar(a *App) tea.Cmd {
	p.fichas = nil
	p.sitioPag = 0
	p.err = nil
	return p.cargar(a, 1)
}

func (p *pantListado) cargar(a *App, pagina int) tea.Cmd {
	if p.cargando && pagina > 1 {
		return nil
	}
	p.cargando = true
	p.pedido++
	pedido := p.pedido
	seccion, archivo, vista := p.seccion, p.archivo, p.vista
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30e9)
		defer cancel()
		var l *site.Listado
		var err error
		if seccion == "" {
			l, err = a.cliente.Portada(ctx, pagina, vista)
		} else {
			l, err = a.cliente.Seccion(ctx, seccion, archivo, pagina, vista)
		}
		return listadoMsg{p, pedido, pagina, l, err}
	}
}

func (p *pantListado) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	m, ok := msg.(listadoMsg)
	if !ok || m.p != p || m.pedido != p.pedido {
		return nil
	}
	p.cargando = false
	if m.err != nil {
		if len(p.fichas) == 0 {
			p.err = m.err
		} else {
			a.fallar(m.err)
		}
		return nil
	}
	a.actualizarCabecera(m.l.Cabecera)
	if m.pagina == 1 {
		p.fichas = []site.Ficha{} // vacío, no nil: nil es "todavía cargando"
	}
	// Entre página y página del sitio pueden subir publicaciones: sin repetir.
	vistas := map[int]bool{}
	for _, f := range p.fichas {
		vistas[f.Hilo] = true
	}
	for _, f := range m.l.Fichas {
		if !vistas[f.Hilo] {
			p.fichas = append(p.fichas, f)
			vistas[f.Hilo] = true
		}
	}
	p.sitioPag = m.l.Pagina
	p.sitioTotal = m.l.Paginas
	p.titulo = m.l.Titulo
	p.descripcion = m.l.Descripcion
	if m.l.Aviso != "" {
		a.avisar(m.l.Aviso)
	}
	if p.selHilo > 0 {
		for i, f := range p.fichas {
			if f.Hilo == p.selHilo {
				p.sel = i
			}
		}
		p.selHilo = 0
	}
	if p.sel >= len(p.fichas) {
		p.sel = max(0, len(p.fichas)-1)
	}
	return nil
}

func (p *pantListado) hayMas() bool { return p.sitioPag < p.sitioTotal }

// ─── Dibujo ─────────────────────────────────────────────────────────────────────────────────

func (p *pantListado) cabecera(a *App, z *Zonas, w int) []Linea {
	es := a.es
	var ls []Linea
	if p.seccion != "" {
		s := site.SeccionPorSlug(p.seccion)
		titulo := p.titulo
		if titulo == "" && s != nil {
			titulo = s.Nombre
			if p.archivo {
				titulo += " · " + T("archivo")
			}
		}
		ls = append(ls, a.h1(titulo, w)...)
		desc := p.descripcion
		if desc == "" && s != nil {
			desc = s.Descripcion
		}
		ls = append(ls, txt(es.texto, recortar(desc, w)))
		link := T("archivo")
		if p.archivo {
			link = T("volver_activas")
		}
		y := len(ls)
		ls = append(ls, txt(es.acento.subrayado(), link))
		z.agregar(0, y, ancho(link), y+1, func() tea.Cmd { return p.alternarArchivo(a) })
	}
	// Publicar (o "Entrá para publicar") y el selector de vista.
	y := len(ls)
	var izq Linea
	if !p.archivo {
		if a.conectado() {
			t := "+ " + mayus(T("publicar"))
			izq = txt(es.acento.negrita(), t)
			z.agregar(0, y, ancho(t), y+1, func() tea.Cmd { return p.publicar(a) })
		} else {
			izq = Linea{{es.acento.subrayado(), T("entrar")}, {es.texto, " — " + T("entra_para")}}
			z.agregar(0, y, ancho(T("entrar")), y+1, func() tea.Cmd { return a.Ir(nuevaEntrar()) })
		}
	}
	der := Linea{{es.tenue, mayus(T("vista")) + " "}}
	for _, v := range []string{"catalogo", "lista"} {
		nombre := mayus(T(v))
		if v == p.vista {
			der = append(der, Run{es.invertido, " " + nombre + " "})
		} else {
			der = append(der, Run{es.acento, " " + nombre + " "})
		}
	}
	x := w - der.Ancho() + ancho(mayus(T("vista"))+" ")
	for _, v := range []string{"catalogo", "lista"} {
		wv := ancho(mayus(T(v))) + 2
		vv := v
		z.agregar(x, y, x+wv, y+1, func() tea.Cmd { return p.cambiarVista(a, vv) })
		x += wv
	}
	ls = append(ls, aDerecha(izq, der, w, es.fondo))
	if p.seccion == "" {
		yy := len(ls)
		l := Linea{{es.tenue, T("moderado") + " "}, {es.acento.subrayado(), T("normas")}}
		if l.Ancho() > w {
			l = txt(es.tenue, recortar(T("moderado"), w))
		} else {
			xn := ancho(T("moderado") + " ")
			z.agregar(xn, yy, xn+ancho(T("normas")), yy+1, func() tea.Cmd { return a.Ir(nuevaDocumento("/normas", T("normas"))) })
		}
		ls = append(ls, l)
	}
	ls = append(ls, nil)
	return ls
}

func (p *pantListado) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	cab := p.cabecera(a, z, w)
	if p.fichas == nil {
		return append(cab, a.estadoCarga(w, alto-len(cab), p.err)...)
	}
	if len(p.fichas) == 0 {
		return rellenar(append(cab, txt(a.es.tenue, T("sin_publicaciones"))), alto)
	}
	disponible := alto - len(cab) - 2 // la paginación y una línea en blanco
	var cuerpo []Linea
	var pag, total int
	if p.vista == "lista" {
		cuerpo, pag, total = p.dibujarLista(a, z, w, disponible, len(cab))
	} else {
		cuerpo, pag, total = p.dibujarCatalogo(a, z, w, disponible, len(cab))
	}
	ls := append(cab, rellenar(cuerpo, disponible)...)
	ls = append(ls, nil)
	y := len(ls)
	ls = append(ls, a.paginacion(z, y, pag+1, total, w, func(n int) tea.Cmd { return p.irAPagina(a, n-1) }))
	return rellenar(ls, alto)
}

func (p *pantListado) dibujarCatalogo(a *App, z *Zonas, w, alto, y0 int) ([]Linea, int, int) {
	gap := 1
	cols := max(1, (w+gap)/(anchoFichaMin+gap))
	cw := (w - gap*(cols-1)) / cols
	filas := max(1, alto/altoFicha)
	// Las fichas se estiran para llenar el alto (hasta 4 líneas más de texto cada una).
	altoF := altoFicha + min(4, (alto-filas*altoFicha)/filas)
	p.cols = cols
	p.porPagina = cols * filas
	pag := p.sel / p.porPagina
	p.pagActual = pag
	desde := pag * p.porPagina
	var ls []Linea
	for f := 0; f < filas; f++ {
		fila := make([]Linea, altoF)
		hay := false
		for c := 0; c < cols; c++ {
			i := desde + f*cols + c
			var cajaF []Linea
			if i < len(p.fichas) {
				hay = true
				cajaF = a.dibujarFicha(p.fichas[i], cw, altoF, i == p.sel)
				idx := i
				x := c * (cw + gap)
				z.agregar(x, y0+f*altoF, x+cw, y0+(f+1)*altoF, func() tea.Cmd {
					if p.sel == idx {
						return p.abrir(a)
					}
					p.sel = idx
					return nil
				})
			}
			for r := 0; r < altoF; r++ {
				var l Linea
				if r < len(cajaF) {
					l = cajaF[r]
				}
				if c > 0 {
					fila[r] = append(fila[r], espacio(a.es.fondo, gap))
				}
				fila[r] = mas(fila[r], completar(l, cw, a.es.fondo))
			}
		}
		if !hay {
			if p.hayMas() && f == 0 {
				ls = append(ls, txt(a.es.tenue, T("cargando")))
			}
			break
		}
		ls = append(ls, fila...)
	}
	total := (len(p.fichas) + p.porPagina - 1) / p.porPagina
	if p.hayMas() {
		estimado := (p.sitioTotal*60 + p.porPagina - 1) / p.porPagina
		total = max(total, estimado)
	}
	return ls, pag, max(1, total)
}

// dibujarLista: la vista lista del sitio (asunto, mensaje inicial, omitidas y últimas respuestas).
func (p *pantListado) dibujarLista(a *App, z *Zonas, w, alto, y0 int) ([]Linea, int, int) {
	bloques := make([][]Linea, len(p.fichas))
	alturas := make([]int, len(p.fichas))
	for i, f := range p.fichas {
		bloques[i] = a.dibujarResumen(f, w, i == p.sel)
		alturas[i] = len(bloques[i])
	}
	p.paginas = paginarAlturas(alturas, alto, 1)
	pag := paginaDe(p.paginas, p.sel)
	p.pagActual = pag
	r := p.paginas[pag]
	var ls []Linea
	for i := r.desde; i < r.hasta; i++ {
		if i > r.desde {
			ls = append(ls, nil)
		}
		y := y0 + len(ls)
		idx := i
		z.agregar(0, y, w, y+len(bloques[i]), func() tea.Cmd {
			if p.sel == idx {
				return p.abrir(a)
			}
			p.sel = idx
			return nil
		})
		ls = append(ls, bloques[i]...)
	}
	total := len(p.paginas)
	if p.hayMas() {
		// Más o menos: las páginas del sitio que faltan, al ritmo de las cargadas.
		porPag := max(1, len(p.fichas)/max(1, len(p.paginas)))
		total += ((p.sitioTotal-p.sitioPag)*10 + porPag - 1) / porPag
	}
	return ls, pag, total
}

// dibujarResumen: un hilo en la vista lista (section.hilo-resumen).
func (a *App) dibujarResumen(f site.Ficha, w int, sel bool) []Linea {
	es := a.es
	var ls []Linea
	tituloE := es.acento.negrita()
	if sel {
		tituloE = es.invertido
	}
	cab := Linea{{es.tenue, "» "}, {tituloE, f.Asunto}}
	if f.Fijada {
		cab = append(cab, Run{es.fondo, " "}, Run{Est{Fg: a.tema.Fondo, Bg: a.tema.Aviso, Negrita: true}, " " + mayus(T("fijada")) + " "})
	}
	if f.Novedad != "" {
		cab = append(cab, Run{es.fondo, " "}, Run{es.acento.negrita(), mayus(f.Novedad)})
	}
	if f.Seccion != "" {
		cab = append(cab, Run{es.fondo, " "}, Run{es.tenue, "[" + mayus(f.Seccion) + "]"})
	}
	ls = append(ls, envolverLinea(cab, w)...)
	if f.Resumen == nil {
		ls = append(ls, txt(es.texto, recortar(f.Extracto, w)))
		return ls
	}
	ls = append(ls, a.dibujarPost(f.Resumen.Op, w, opcionesPost{maxTexto: 4})...)
	if f.Resumen.Omitidas > 0 {
		t := T("omitidas_n", f.Resumen.Omitidas)
		if f.Resumen.Omitidas == 1 {
			t = T("omitidas_1")
		}
		ls = append(ls, txt(es.tenue, "   … "+t))
	}
	for _, r := range f.Resumen.Ultimas {
		ls = append(ls, a.dibujarPost(r, w, opcionesPost{maxTexto: 2})...)
	}
	pie := f.Resumen.Pie
	if pie == "" {
		pie = itoa(f.R) + " respuestas"
	}
	ls = append(ls, txt(es.tenue, recortar(pie, w)))
	return ls
}

// ─── Teclas ─────────────────────────────────────────────────────────────────────────────────

func (p *pantListado) Teclas(a *App) []Atajo {
	at := []Atajo{{"↑↓←→", T("k_mover")}, {"Enter", T("k_abrir")}, {"PgDn", T("k_pagina")}, {"Tab", T("k_seccion")}}
	if !p.archivo {
		at = append(at, Atajo{"n", T("k_publicar")})
	}
	at = append(at, Atajo{"v", T("k_vista")}, Atajo{"/", T("k_buscar")}, Atajo{"m", T("k_menu")}, Atajo{"?", T("k_ayuda")}, Atajo{"q", T("k_salir")})
	return at
}

func (p *pantListado) mover(a *App, d int) tea.Cmd {
	if len(p.fichas) == 0 {
		return nil
	}
	n := p.sel + d
	if n < 0 {
		n = 0
	}
	var cmd tea.Cmd
	if n >= len(p.fichas) {
		if p.hayMas() {
			cmd = p.cargar(a, p.sitioPag+1)
		}
		n = len(p.fichas) - 1
	}
	p.sel = n
	// Acercándose al final de lo cargado: pedir la página siguiente del sitio.
	if cmd == nil && p.hayMas() && p.porPagina > 0 && p.sel+p.porPagina >= len(p.fichas) {
		cmd = p.cargar(a, p.sitioPag+1)
	}
	return cmd
}

func (p *pantListado) cambiarPagina(a *App, d int) tea.Cmd {
	if p.vista == "lista" {
		pags := p.paginas
		if len(pags) == 0 {
			return nil
		}
		pag := paginaDe(pags, p.sel) + d
		if pag < 0 {
			pag = 0
		}
		if pag >= len(pags) {
			if p.hayMas() {
				p.sel = len(p.fichas) - 1
				return p.cargar(a, p.sitioPag+1)
			}
			pag = len(pags) - 1
		}
		p.sel = pags[pag].desde
		return p.mover(a, 0)
	}
	if p.porPagina == 0 {
		return nil
	}
	col := p.sel % p.porPagina
	destino := (p.pagActual+d)*p.porPagina + col
	if destino < 0 {
		destino = 0
	}
	return p.mover(a, destino-p.sel)
}

func (p *pantListado) irAPagina(a *App, pag int) tea.Cmd {
	if p.vista == "lista" {
		return p.cambiarPagina(a, pag-p.pagActual)
	}
	return p.mover(a, pag*p.porPagina-p.sel)
}

func (p *pantListado) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	lista := p.vista == "lista"
	switch k.String() {
	case "left":
		if lista {
			return true, p.cambiarPagina(a, -1)
		}
		return true, p.mover(a, -1)
	case "right":
		if lista {
			return true, p.cambiarPagina(a, 1)
		}
		return true, p.mover(a, 1)
	case "up", "k":
		if lista {
			return true, p.mover(a, -1)
		}
		return true, p.mover(a, -max(1, p.cols))
	case "down", "j":
		if lista {
			return true, p.mover(a, 1)
		}
		return true, p.mover(a, max(1, p.cols))
	case "pgdown", "space", "]":
		return true, p.cambiarPagina(a, 1)
	case "pgup", "[":
		return true, p.cambiarPagina(a, -1)
	case "home":
		p.sel = 0
		return true, nil
	case "end":
		return true, p.mover(a, len(p.fichas))
	case "enter":
		return true, p.abrir(a)
	case "n":
		if p.archivo {
			return true, nil
		}
		return true, p.publicar(a)
	case "v":
		otra := "lista"
		if p.vista == "lista" {
			otra = "catalogo"
		}
		return true, p.cambiarVista(a, otra)
	case "a":
		if p.seccion != "" {
			return true, p.alternarArchivo(a)
		}
	case "w":
		if f := p.elegida(); f != nil {
			abrirNavegador(site.BaseURL + "/h/" + itoa(f.Hilo))
		}
		return true, nil
	case "esc", "backspace":
		if p.seccion != "" || p.archivo {
			if len(a.pila) > 1 {
				return true, a.Volver()
			}
			return true, a.Inicio()
		}
		return true, nil
	}
	return false, nil
}

func (p *pantListado) elegida() *site.Ficha {
	if p.sel >= 0 && p.sel < len(p.fichas) {
		return &p.fichas[p.sel]
	}
	return nil
}

func (p *pantListado) abrir(a *App) tea.Cmd {
	f := p.elegida()
	if f == nil {
		return nil
	}
	// Abrirla la marca como vista en el sitio; al volver se recarga lo nuevo.
	f.Novedad = ""
	return a.Ir(nuevaHilo(f.Hilo, 0))
}

func (p *pantListado) publicar(a *App) tea.Cmd {
	if !a.conectado() {
		return a.Ir(nuevaEntrar())
	}
	return a.Ir(nuevaComposerHilo(a, p.seccion))
}

func (p *pantListado) cambiarVista(a *App, v string) tea.Cmd {
	if v == p.vista {
		return nil
	}
	p.vista = v
	a.prefs.Vista = v
	config.GuardarPreferencias(a.prefs)
	// El sitio da distinta cantidad por página: se vuelve a cargar, cerca de donde estaba.
	p.sel = 0
	return p.Recargar(a)
}

func (p *pantListado) alternarArchivo(a *App) tea.Cmd {
	return a.Reemplazar(nuevaListado(p.seccion, !p.archivo, p.vista))
}

// AlVolver: al volver de una publicación, se refrescan las marcas y los R.
func (p *pantListado) AlVolver(a *App) tea.Cmd {
	if f := p.elegida(); f != nil {
		p.selHilo = f.Hilo
	}
	p.pedido++
	pedido := p.pedido
	paginas := max(1, p.sitioPag)
	seccion, archivo, vista := p.seccion, p.archivo, p.vista
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30e9)
		defer cancel()
		var todas []site.Ficha
		var ultimo *site.Listado
		for pg := 1; pg <= paginas; pg++ {
			var l *site.Listado
			var err error
			if seccion == "" {
				l, err = a.cliente.Portada(ctx, pg, vista)
			} else {
				l, err = a.cliente.Seccion(ctx, seccion, archivo, pg, vista)
			}
			if err != nil {
				return listadoMsg{p, pedido, pg, nil, err}
			}
			todas = append(todas, l.Fichas...)
			ultimo = l
		}
		ultimo.Fichas = todas
		ultimo.Pagina = paginas
		return listadoMsg{p, pedido, 1, ultimo, nil}
	}
}
