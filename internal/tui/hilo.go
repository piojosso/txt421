package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// Cada cuánto se piden los mensajes nuevos (public/vivo.js: cada 20 segundos).
const cadaVivo = 20 * time.Second

// pantHilo: una publicación, repartida en páginas del alto de la ventana.
type pantHilo struct {
	no       int
	irA      int // No. de mensaje a elegir al cargar
	porPost  int // se abrió por un No. de mensaje: hay que averiguar la publicación
	hilo     *site.Hilo
	err      error
	cargando bool
	pedido   int

	sel       int  // mensaje elegido (índice en hilo.Posts)
	pag       int  // página que se ve
	destapar  bool // spoilers del elegido, visibles
	historial []int
	citaIdx   int // para recorrer las citas del elegido con i
	avisoVivo string
	vivo      bool // hay un tick andando
	guardando bool

	// Del último dibujo.
	paginas [][]segmento
}

// segmento: un mensaje o un pedazo de un mensaje largo.
type segmento struct {
	post   int
	lineas []Linea
	sigue  bool // el mensaje sigue en la página siguiente
	viene  bool // viene de la página anterior
}

type hiloMsg struct {
	p      *pantHilo
	pedido int
	h      *site.Hilo
	err    error
}
type ubicadoMsg struct {
	p    *pantHilo
	hilo int
	err  error
}
type vivoTick struct{ p *pantHilo }
type vivoMsg struct {
	p      *pantHilo
	ultimo int
	posts  []site.Post
	err    error
}
type guardadoMsg struct {
	p   *pantHilo
	err error
}

func nuevaHilo(no, irA int) *pantHilo { return &pantHilo{no: no, irA: irA} }

func nuevaHiloPorPost(post int) *pantHilo { return &pantHilo{porPost: post, irA: post} }

func (p *pantHilo) Titulo() string {
	if p.hilo != nil {
		return p.hilo.Asunto
	}
	return "…"
}

func (p *pantHilo) SeccionActiva() string {
	if p.hilo != nil {
		return p.hilo.Seccion
	}
	return ""
}

func (p *pantHilo) Recargar(a *App) tea.Cmd {
	p.err = nil
	p.cargando = true
	p.pedido++
	pedido := p.pedido
	if p.no == 0 && p.porPost > 0 {
		post := p.porPost
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
			defer cancel()
			h, err := a.cliente.Ubicar(ctx, post)
			return ubicadoMsg{p, h, err}
		}
	}
	no := p.no
	cmds := []tea.Cmd{func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		h, err := a.cliente.Hilo(ctx, no)
		return hiloMsg{p, pedido, h, err}
	}}
	if !p.vivo {
		p.vivo = true
		cmds = append(cmds, p.tickVivo())
	}
	return tea.Batch(cmds...)
}

func (p *pantHilo) tickVivo() tea.Cmd {
	return tea.Tick(cadaVivo, func(time.Time) tea.Msg { return vivoTick{p} })
}

func (p *pantHilo) Mensaje(a *App, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case ubicadoMsg:
		if m.p != p {
			return nil
		}
		if m.err != nil {
			p.cargando = false
			p.err = m.err
			return nil
		}
		p.no = m.hilo
		return p.Recargar(a)
	case hiloMsg:
		if m.p != p || m.pedido != p.pedido {
			return nil
		}
		p.cargando = false
		if m.err != nil {
			if p.hilo == nil {
				p.err = m.err
			} else {
				a.fallar(m.err)
			}
			return nil
		}
		a.actualizarCabecera(m.h.Cabecera)
		anterior := -1
		if p.hilo != nil && p.sel < len(p.hilo.Posts) {
			anterior = p.hilo.Posts[p.sel].No
		}
		p.hilo = m.h
		if m.h.Aviso != "" {
			a.avisar(m.h.Aviso)
		}
		p.avisoVivo = ""
		switch {
		case p.irA > 0:
			p.elegirNo(p.irA)
			p.irA = 0
		case anterior >= 0:
			p.elegirNo(anterior)
		default:
			// Si hay mensajes nuevos desde tu visita anterior, se arranca en el primero.
			p.sel = 0
			for i, post := range m.h.Posts {
				if post.Nuevo && !post.Inicial {
					p.sel = i
					break
				}
			}
		}
		p.pag = -1 // que el dibujo ubique la página del elegido
		return nil
	case vivoTick:
		if m.p != p {
			return nil
		}
		// Solo mientras se la está mirando y acepta respuestas (como la web con la pestaña visible).
		if a.actual() != p || a.capa != nil || p.hilo == nil || !p.hilo.Abierto {
			if a.estaEnPila(p) {
				return p.tickVivo()
			}
			p.vivo = false
			return nil
		}
		no, desde := p.hilo.No, p.hilo.Ultimo
		return tea.Batch(p.tickVivo(), func() tea.Msg {
			ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
			defer cancel()
			u, posts, err := a.cliente.Nuevos(ctx, no, desde)
			return vivoMsg{p, u, posts, err}
		})
	case vivoMsg:
		if m.p != p || m.err != nil || p.hilo == nil {
			return nil
		}
		agregados := 0
		for _, post := range m.posts {
			if post.No <= p.hilo.Ultimo {
				continue
			}
			post.Nuevo = true
			p.hilo.Posts = append(p.hilo.Posts, post)
			agregados++
		}
		p.hilo.Ultimo = max(p.hilo.Ultimo, m.ultimo)
		if agregados > 0 {
			p.avisoVivo = T("hay_nuevos", len(p.hilo.Posts))
		}
		return nil
	case guardadoMsg:
		if m.p != p {
			return nil
		}
		p.guardando = false
		if m.err != nil {
			a.fallar(m.err)
			return nil
		}
		return p.Recargar(a)
	}
	return nil
}

func (a *App) estaEnPila(p Pantalla) bool {
	for _, x := range a.pila {
		if x == p {
			return true
		}
	}
	return false
}

func (p *pantHilo) elegirNo(no int) {
	for i, post := range p.hilo.Posts {
		if post.No == no {
			p.sel = i
			return
		}
	}
}

// ─── Dibujo ─────────────────────────────────────────────────────────────────────────────────

func (p *pantHilo) cabecera(a *App, z *Zonas, w int) []Linea {
	es := a.es
	h := p.hilo
	var ls []Linea
	// "← Vida real · Guardar · Fijada"
	var arriba Linea
	x := 0
	if s := site.SeccionPorSlug(h.Seccion); s != nil {
		t := "← " + s.Nombre
		arriba = append(arriba, Run{es.acento.subrayado(), t})
		slug := s.Slug
		z.agregar(0, 0, ancho(t), 1, func() tea.Cmd { return a.abrirSeccion(slug) })
		x += ancho(t)
	}
	if h.Fijada {
		arriba = append(arriba, Run{es.fondo, " "}, Run{Est{Fg: a.tema.Fondo, Bg: a.tema.Aviso, Negrita: true}, " " + mayus(T("fijada")) + " "})
		x += ancho(T("fijada")) + 3
	}
	if h.PuedeGuardar {
		t := mayus(T("guardar"))
		if h.Guardado {
			t = mayus(T("sacar_guardados"))
		}
		arriba = append(arriba, Run{es.tenue, " · "}, Run{es.acento.negrita(), t})
		x += 3
		z.agregar(x, 0, x+ancho(t), 1, func() tea.Cmd { return p.guardar(a) })
	}
	ls = append(ls, arriba)
	titulo := a.h1(h.Asunto, w)
	if len(titulo) > 2 {
		titulo = cortarConElipsis(titulo, 2, w)
	}
	ls = append(ls, titulo...)
	if h.EstadoTxt != "" {
		ls = append(ls, txt(es.aviso, recortar("[!] "+h.EstadoTxt, w)))
	}
	ls = append(ls, nil)
	return ls
}

// armarPaginas corta los mensajes en segmentos y los reparte en páginas de alto líneas.
func (p *pantHilo) armarPaginas(a *App, w, alto int) {
	var segs []segmento
	for i, post := range p.hilo.Posts {
		ls := a.dibujarPost(post, w, opcionesPost{sel: i == p.sel, destapar: i == p.sel && p.destapar})
		if len(ls) <= alto {
			segs = append(segs, segmento{post: i, lineas: ls})
			continue
		}
		// Mensaje más largo que la página: en pedazos con "(sigue)".
		trozo := max(3, alto-1)
		for j := 0; j < len(ls); j += trozo {
			fin := min(len(ls), j+trozo)
			segs = append(segs, segmento{post: i, lineas: ls[j:fin], viene: j > 0, sigue: fin < len(ls)})
		}
	}
	alturas := make([]int, len(segs))
	for i, s := range segs {
		alturas[i] = len(s.lineas)
		if s.sigue {
			alturas[i]++
		}
	}
	p.paginas = nil
	for _, r := range paginarAlturas(alturas, alto, 1) {
		p.paginas = append(p.paginas, segs[r.desde:r.hasta])
	}
}

// paginaDelElegido: la primera página donde aparece el mensaje elegido.
func (p *pantHilo) paginaDelElegido() int {
	for i, pg := range p.paginas {
		for _, s := range pg {
			if s.post == p.sel {
				return i
			}
		}
	}
	return 0
}

func (p *pantHilo) enPagina(pag, post int) bool {
	if pag < 0 || pag >= len(p.paginas) {
		return false
	}
	for _, s := range p.paginas[pag] {
		if s.post == post {
			return true
		}
	}
	return false
}

func (p *pantHilo) Dibujar(a *App, z *Zonas, w, alto int) []Linea {
	if p.hilo == nil {
		return a.estadoCarga(w, alto, p.err)
	}
	es := a.es
	cab := p.cabecera(a, z, w)
	// Abajo: el aviso de mensajes nuevos, la invitación a responder y la paginación.
	pie := 2
	if p.avisoVivo != "" {
		pie++
	}
	disponible := max(4, alto-len(cab)-pie)
	p.armarPaginas(a, w, disponible)
	if p.pag < 0 || p.pag >= len(p.paginas) || !p.enPagina(p.pag, p.sel) {
		p.pag = p.paginaDelElegido()
	}
	ls := cab
	for i, s := range p.paginas[p.pag] {
		if i > 0 {
			ls = append(ls, nil)
		}
		y := len(ls)
		idx := s.post
		z.agregar(0, y, w, y+len(s.lineas), func() tea.Cmd {
			if p.sel == idx {
				return p.responder(a, true)
			}
			p.sel, p.destapar, p.citaIdx = idx, false, 0
			return nil
		})
		ls = append(ls, s.lineas...)
		if s.sigue {
			ls = append(ls, txt(es.tenue, "   "+T("sigue")+" →"))
		}
	}
	ls = rellenar(ls, alto-pie)
	if p.avisoVivo != "" {
		ls = append(ls, txt(es.tenue, recortar(p.avisoVivo, w)))
	}
	// Última página: invitación a responder (o a entrar).
	var cta Linea
	y := len(ls)
	switch {
	case !p.hilo.Abierto:
	case a.conectado():
		b := a.boton(T("responder"), false)
		cta = mas(b, Linea{{es.tenue, "  r · Enter " + T("k_citar") + " >>" + itoa(p.elegido().No)}})
		z.agregar(0, y, b.Ancho(), y+1, func() tea.Cmd { return p.responder(a, false) })
	default:
		cta = Linea{{es.acento.subrayado(), T("entrar")}, {es.texto, " — " + T("entra_responder")}}
		z.agregar(0, y, ancho(T("entrar")), y+1, func() tea.Cmd { return a.Ir(nuevaEntrar()) })
	}
	pagL := a.paginacion(z, y, p.pag+1, len(p.paginas), max(10, w-cta.Ancho()-2), func(n int) tea.Cmd {
		p.irPagina(n - 1)
		return nil
	})
	ls = append(ls, aDerecha(cta, pagL, w, es.fondo))
	return rellenar(ls, alto)
}

func (p *pantHilo) elegido() site.Post {
	if p.hilo == nil || p.sel >= len(p.hilo.Posts) {
		return site.Post{}
	}
	return p.hilo.Posts[p.sel]
}

// ─── Teclas ─────────────────────────────────────────────────────────────────────────────────

func (p *pantHilo) Teclas(a *App) []Atajo {
	at := []Atajo{{"↑↓", T("k_mover")}, {"←→", T("k_pagina")}}
	e := p.elegido()
	if p.hilo != nil && p.hilo.Abierto {
		at = append(at, Atajo{"Enter", T("k_citar")}, Atajo{"r", T("k_responder")})
	}
	if tieneCitas(e) {
		at = append(at, Atajo{"i", T("k_ir_cita")})
	}
	if tieneSpoiler(e) {
		at = append(at, Atajo{"s", "spoiler"})
	}
	if p.hilo != nil && p.hilo.PuedeGuardar {
		at = append(at, Atajo{"g", T("k_guardar")})
	}
	if e.PuedeBorrar {
		at = append(at, Atajo{"x", T("borrar")})
	} else if e.PuedeReportar {
		at = append(at, Atajo{"x", T("reportar")})
	}
	at = append(at, Atajo{"w", "web"}, Atajo{"esc", T("k_volver")}, Atajo{"?", T("k_ayuda")})
	return at
}

func tieneCitas(p site.Post) bool {
	if len(p.Respuestas) > 0 {
		return true
	}
	for _, t := range p.Cuerpo {
		if t.Estilo == site.Ref {
			return true
		}
	}
	return false
}

func tieneSpoiler(p site.Post) bool {
	for _, t := range p.Cuerpo {
		if t.Estilo == site.Spoiler {
			return true
		}
	}
	return false
}

func (p *pantHilo) mover(d int) {
	if p.hilo == nil {
		return
	}
	n := p.sel + d
	n = max(0, min(n, len(p.hilo.Posts)-1))
	if n != p.sel {
		p.sel, p.destapar, p.citaIdx = n, false, 0
	}
}

func (p *pantHilo) irPagina(pag int) {
	if len(p.paginas) == 0 {
		return
	}
	pag = max(0, min(pag, len(p.paginas)-1))
	p.pag = pag
	if !p.enPagina(pag, p.sel) {
		p.sel = p.paginas[pag][0].post
		p.destapar, p.citaIdx = false, 0
	}
}

func (p *pantHilo) Tecla(a *App, k tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.hilo == nil {
		return false, nil
	}
	switch k.String() {
	case "up", "k":
		p.mover(-1)
	case "down", "j":
		p.mover(1)
	case "left", "pgup", "[":
		p.irPagina(p.pag - 1)
	case "right", "pgdown", "space", "]":
		p.irPagina(p.pag + 1)
	case "home":
		p.mover(-len(p.hilo.Posts))
	case "g":
		if !p.hilo.PuedeGuardar {
			return false, nil
		}
		return true, p.guardar(a)
	case "end":
		p.mover(len(p.hilo.Posts))
	case "enter":
		return true, p.responder(a, true)
	case "r":
		if p.hilo.Abierto && a.conectado() {
			return true, p.responder(a, false)
		}
		return true, p.Recargar(a)
	case "f5":
		return true, p.Recargar(a)
	case "s":
		p.destapar = !p.destapar
	case "i":
		return true, p.irACita(a)
	case "x":
		return true, p.reportarOBorrar(a)
	case "w":
		e := p.elegido()
		abrirNavegador(site.BaseURL + "/h/" + itoa(p.hilo.No) + "#p" + itoa(e.No))
	case "esc", "backspace":
		if len(p.historial) > 0 {
			p.sel = p.historial[len(p.historial)-1]
			p.historial = p.historial[:len(p.historial)-1]
			p.destapar, p.citaIdx = false, 0
			return true, nil
		}
		return true, a.Volver()
	default:
		return false, nil
	}
	return true, nil
}

// irACita salta al mensaje citado (>>N en el texto o en "Respuestas:"); repetir recorre las citas.
func (p *pantHilo) irACita(a *App) tea.Cmd {
	e := p.elegido()
	var nums []int
	for _, t := range e.Cuerpo {
		if t.Estilo == site.Ref {
			nums = append(nums, t.Num)
		}
	}
	nums = append(nums, e.Respuestas...)
	if len(nums) == 0 {
		a.avisar(T("cita_no_hay"))
		return nil
	}
	n := nums[p.citaIdx%len(nums)]
	p.citaIdx++
	for i, post := range p.hilo.Posts {
		if post.No == n {
			p.historial = append(p.historial, p.sel)
			p.sel, p.destapar = i, false
			p.citaIdx = 0
			return nil
		}
	}
	a.avisar(T("cita_fuera", n))
	return a.Ir(nuevaHiloPorPost(n))
}

func (p *pantHilo) responder(a *App, citar bool) tea.Cmd {
	if !p.hilo.Abierto {
		a.avisar(T("no_acepta"))
		return nil
	}
	if !a.conectado() {
		return a.Ir(nuevaEntrar())
	}
	cita := 0
	// Como en el sitio: los retirados y los que están en revisión no tienen "Responder".
	if e := p.elegido(); citar && e.Retirado == "" && !e.EnRevision {
		cita = e.No
	}
	return a.Ir(nuevaComposerRespuesta(a, p, cita))
}

func (p *pantHilo) guardar(a *App) tea.Cmd {
	if !p.hilo.PuedeGuardar || p.guardando {
		return nil
	}
	p.guardando = true
	no, quitar := p.hilo.No, p.hilo.Guardado
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		return guardadoMsg{p, a.cliente.Guardar(ctx, no, quitar)}
	}
}

func (p *pantHilo) reportarOBorrar(a *App) tea.Cmd {
	e := p.elegido()
	switch {
	case e.PuedeBorrar:
		a.capa = nuevoDialogoBorrar(a, p, e)
		return a.capa.(*dialogoBorrar).cargar(a)
	case e.PuedeReportar:
		a.capa = nuevoDialogoReportar(a, p, e)
		return a.capa.(*dialogoReportar).cargar(a)
	case !a.conectado():
		return a.Ir(nuevaEntrar())
	}
	return nil
}

// alPublicar: después de responder, se recarga eligiendo el mensaje nuevo.
func (p *pantHilo) alPublicar(a *App, post int) tea.Cmd {
	if post > 0 {
		p.irA = post
	} else if p.hilo != nil && len(p.hilo.Posts) > 0 {
		p.irA = -1
	}
	return p.Recargar(a)
}

var _ = errors.New
