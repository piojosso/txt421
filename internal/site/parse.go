package site

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ─── Árbol ──────────────────────────────────────────────────────────────────────────────────

func attr(n *html.Node, k string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

func tieneAttr(n *html.Node, k string) bool {
	for _, a := range n.Attr {
		if a.Key == k {
			return true
		}
	}
	return false
}

func tieneClase(n *html.Node, c string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, x := range strings.Fields(attr(n, "class")) {
		if x == c {
			return true
		}
	}
	return false
}

type filtro func(*html.Node) bool

func tag(t string) filtro {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == t }
}
func clase(c string) filtro { return func(n *html.Node) bool { return tieneClase(n, c) } }
func id(v string) filtro {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && attr(n, "id") == v }
}
func y(fs ...filtro) filtro {
	return func(n *html.Node) bool {
		for _, f := range fs {
			if !f(n) {
				return false
			}
		}
		return true
	}
}

// todos devuelve los descendientes (no el propio nodo) que cumplen el filtro, en orden.
func todos(n *html.Node, f filtro) []*html.Node {
	var out []*html.Node
	if n == nil {
		return out
	}
	var rec func(*html.Node)
	rec = func(p *html.Node) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			if f(c) {
				out = append(out, c)
			}
			rec(c)
		}
	}
	rec(n)
	return out
}

func uno(n *html.Node, f filtro) *html.Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f(c) {
			return c
		}
		if r := uno(c, f); r != nil {
			return r
		}
	}
	return nil
}

// hijos devuelve los hijos elemento directos.
func hijos(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

// texto junta el texto de un nodo; <br> es salto de línea.
func texto(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(p *html.Node) {
		switch {
		case p.Type == html.TextNode:
			b.WriteString(p.Data)
		case p.Type == html.ElementNode && p.Data == "br":
			b.WriteByte('\n')
		case p.Type == html.ElementNode && (p.Data == "script" || p.Data == "style" || p.Data == "svg"):
		default:
			for c := p.FirstChild; c != nil; c = c.NextSibling {
				rec(c)
			}
		}
	}
	rec(n)
	return b.String()
}

var espacios = regexp.MustCompile(`\s+`)

func limpio(s string) string { return strings.TrimSpace(espacios.ReplaceAllString(s, " ")) }

var numero = regexp.MustCompile(`\d+`)

func num(s string) int {
	m := numero.FindString(s)
	n, _ := strconv.Atoi(m)
	return n
}

func parsear(doc string) *html.Node {
	n, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return &html.Node{Type: html.DocumentNode}
	}
	return n
}

// ─── Texto de los mensajes ──────────────────────────────────────────────────────────────────

// tramos convierte el HTML de un mensaje (div.texto) en tramos.
func tramos(n *html.Node) []Tramo {
	var out []Tramo
	var rec func(p *html.Node, e Estilo)
	agregar := func(e Estilo, s string) {
		// Saltos de línea sueltos en el texto (fragmentos de búsqueda con white-space: pre-wrap).
		partes := strings.Split(s, "\n")
		for i, p := range partes {
			if i > 0 {
				out = append(out, Tramo{Estilo: Salto})
			}
			if p != "" {
				out = append(out, Tramo{Estilo: e, Texto: p})
			}
		}
	}
	rec = func(p *html.Node, e Estilo) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				agregar(e, c.Data)
			case c.Type != html.ElementNode:
			case c.Data == "br":
				out = append(out, Tramo{Estilo: Salto})
			case c.Data == "a" && tieneClase(c, "cita"):
				t := texto(c)
				out = append(out, Tramo{Estilo: Ref, Texto: t, Num: num(t)})
			case tieneClase(c, "spoiler"):
				out = append(out, Tramo{Estilo: Spoiler, Texto: texto(c)})
			case c.Data == "mark":
				out = append(out, Tramo{Estilo: Marca, Texto: texto(c)})
			case tieneClase(c, "verde"):
				rec(c, Cita)
			default:
				rec(c, e)
			}
		}
	}
	if n != nil {
		rec(n, Normal)
	}
	// Sin saltos al final.
	for len(out) > 0 && out[len(out)-1].Estilo == Salto {
		out = out[:len(out)-1]
	}
	return out
}

// TextoPlano devuelve los tramos como se escriben en el sitio (con [spoiler]…[/spoiler]).
func TextoPlano(ts []Tramo) string {
	var b strings.Builder
	for _, t := range ts {
		switch t.Estilo {
		case Salto:
			b.WriteByte('\n')
		case Spoiler:
			b.WriteString("[spoiler]" + t.Texto + "[/spoiler]")
		default:
			b.WriteString(t.Texto)
		}
	}
	return b.String()
}

// ─── Fechas ─────────────────────────────────────────────────────────────────────────────────

// ZonaSitio es la zona horaria en la que el sitio muestra las fechas.
var ZonaSitio = func() *time.Location {
	if l, err := time.LoadLocation("America/Argentina/Buenos_Aires"); err == nil {
		return l
	}
	return time.FixedZone("ART", -3*3600)
}()

// FechaCorta formatea como el sitio (Intl es-AR, dateStyle short, timeStyle short):
// "29/9/26, 11:53 p. m.".
func FechaCorta(t time.Time) string {
	t = t.In(ZonaSitio)
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	ap := "a. m."
	if t.Hour() >= 12 {
		ap = "p. m."
	}
	return strconv.Itoa(t.Day()) + "/" + strconv.Itoa(int(t.Month())) + "/" + strconv.Itoa(t.Year()%100) +
		", " + strconv.Itoa(h) + ":" + dos(t.Minute()) + " " + ap
}

func dos(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

var fechaSitio = regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{2,4}),\s*(\d{1,2}):(\d{2})\s*([ap])\.?\s*m\.?`)

// LeerFecha interpreta una fecha como la muestra el sitio. Cero si no se entiende.
func LeerFecha(s string) time.Time {
	s = strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(s)
	m := fechaSitio.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}
	}
	d, _ := strconv.Atoi(m[1])
	mes, _ := strconv.Atoi(m[2])
	a, _ := strconv.Atoi(m[3])
	if len(m[3]) == 2 {
		a += 2000
	}
	h, _ := strconv.Atoi(m[4])
	mi, _ := strconv.Atoi(m[5])
	h %= 12
	if strings.EqualFold(m[6], "p") {
		h += 12
	}
	return time.Date(a, time.Month(mes), d, h, mi, 0, 0, ZonaSitio)
}

func normalizarFecha(s string) string {
	return strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(limpio(s))
}

// ─── Cabecera ───────────────────────────────────────────────────────────────────────────────

func leerCabecera(doc *html.Node) Cabecera {
	var c Cabecera
	cab := uno(doc, y(tag("header"), clase("cabecera")))
	if cab == nil {
		return c
	}
	c.Conectado = uno(cab, y(tag("a"), func(n *html.Node) bool { return attr(n, "href") == "/respuestas" })) != nil
	if sobre := uno(cab, clase("correo")); sobre != nil {
		if b := uno(sobre, clase("badge")); b != nil {
			c.Novedades = num(texto(b))
		}
	}
	if mod := uno(cab, clase("mod-pendientes")); mod != nil {
		if b := uno(mod, clase("badge")); b != nil {
			c.Pendientes = num(texto(b))
		}
	}
	if in := uno(doc, y(tag("input"), func(n *html.Node) bool { return attr(n, "name") == "_csrf" })); in != nil {
		c.CSRF = attr(in, "value")
	}
	return c
}

func leerAviso(doc *html.Node) string {
	if m := uno(doc, tag("main")); m != nil {
		for _, h := range hijos(m) {
			if h.Data == "p" && tieneClase(h, "aviso") {
				return limpio(texto(h))
			}
		}
	}
	return ""
}

// ─── Listados ───────────────────────────────────────────────────────────────────────────────

func paginacion(doc *html.Node) (int, int) {
	nav := uno(doc, y(tag("nav"), clase("paginas")))
	if nav == nil {
		return 1, 1
	}
	actual := 1
	if s := uno(nav, tag("strong")); s != nil {
		actual = max(1, num(texto(s)))
	}
	ultima := actual
	for _, a := range todos(nav, tag("a")) {
		if n := num(texto(a)); n > ultima {
			ultima = n
		}
	}
	return actual, ultima
}

var rN = regexp.MustCompile(`R:\s*(\d+)`)

// ParseListado lee la portada, una sección o un archivo, en vista catálogo o lista.
func ParseListado(doc string) *Listado {
	raiz := parsear(doc)
	main := uno(raiz, tag("main"))
	l := &Listado{Cabecera: leerCabecera(raiz), Vista: "catalogo", Aviso: leerAviso(raiz)}
	if v := uno(main, y(tag("nav"), clase("vistas"))); v != nil {
		if s := uno(v, tag("strong")); s != nil && strings.Contains(limpio(texto(s)), "lista") {
			l.Vista = "lista"
		}
	}
	if cab := uno(main, y(tag("header"), clase("tablon-cabecera"))); cab != nil {
		l.Titulo = limpio(texto(uno(cab, tag("h1"))))
		if p := uno(cab, tag("p")); p != nil {
			l.Descripcion = limpio(texto(p))
		}
	}
	for _, art := range todos(main, y(tag("article"), clase("ficha"))) {
		a := uno(art, clase("ficha-asunto"))
		if a == nil {
			continue
		}
		f := Ficha{Hilo: num(attr(a, "href")), Asunto: limpio(texto(a)), Fijada: tieneClase(art, "fijada")}
		if barra := uno(art, clase("ficha-barra")); barra != nil {
			spans := hijos(barra)
			if len(spans) > 0 {
				cabeza := limpio(texto(spans[0]))
				if strings.HasPrefix(cabeza, "No.") {
					f.OpNo = num(cabeza)
				} else {
					f.Seccion = cabeza
				}
			}
			if m := rN.FindStringSubmatch(texto(barra)); m != nil {
				f.R, _ = strconv.Atoi(m[1])
			}
			if nv := uno(barra, clase("marca-nuevo")); nv != nil {
				f.Novedad = limpio(texto(nv))
			}
		}
		if t := uno(art, clase("ficha-texto")); t != nil {
			f.Extracto = limpio(texto(t))
		}
		if p := uno(art, clase("ficha-pie")); p != nil {
			pie := normalizarFecha(texto(p))
			if strings.HasSuffix(pie, " · cerrada") {
				f.Cerrada = true
				pie = strings.TrimSuffix(pie, " · cerrada")
			}
			f.Fecha = pie
		}
		l.Fichas = append(l.Fichas, f)
	}
	for _, sec := range todos(main, y(tag("section"), clase("hilo-resumen"))) {
		h2 := uno(sec, tag("h2"))
		a := uno(h2, tag("a"))
		if a == nil {
			continue
		}
		f := Ficha{Hilo: num(attr(a, "href")), Asunto: limpio(texto(a)), Fijada: tieneClase(sec, "fijada"), Resumen: &Resumen{}}
		if nv := uno(h2, clase("marca-nuevo")); nv != nil {
			f.Novedad = limpio(texto(nv))
		}
		if et := uno(h2, clase("etiqueta")); et != nil {
			f.Resumen.Etiqueta = limpio(texto(et))
			f.Seccion = f.Resumen.Etiqueta
		}
		posts := leerPosts(sec)
		if len(posts) > 0 {
			posts[0].Inicial = true
			f.Resumen.Op = posts[0]
			f.Resumen.Ultimas = posts[1:]
			f.Extracto = limpio(TextoPlano(posts[0].Cuerpo))
			f.Fecha = posts[0].Fecha
		}
		if om := uno(sec, clase("omitidas")); om != nil {
			f.Resumen.Omitidas = num(texto(om))
		}
		for _, p := range todos(sec, y(tag("p"), clase("ayuda"))) {
			t := limpio(texto(p))
			if m := regexp.MustCompile(`(\d+) respuestas`).FindStringSubmatch(t); m != nil {
				f.R, _ = strconv.Atoi(m[1])
				f.Cerrada = strings.Contains(t, "cerrada")
				f.Resumen.Pie = t
			}
		}
		l.Fichas = append(l.Fichas, f)
	}
	l.Pagina, l.Paginas = paginacion(main)
	return l
}

// ─── Publicaciones ──────────────────────────────────────────────────────────────────────────

func leerPosts(raiz *html.Node) []Post {
	var posts []Post
	for _, art := range todos(raiz, y(tag("article"), clase("post"))) {
		if tieneClase(art, "flotante") || tieneClase(art, "desplegada") {
			continue
		}
		p := Post{No: num(attr(art, "id")), Inicial: tieneClase(art, "op"), EnRevision: tieneClase(art, "en-revision")}
		meta := uno(art, clase("post-meta"))
		if meta == nil {
			// Retirado: "No.N · Eliminado por su autor."
			t := limpio(texto(art))
			if i := strings.Index(t, "·"); i >= 0 {
				t = strings.TrimSpace(t[i+len("·"):])
			}
			if p.No == 0 {
				p.No = num(t)
			}
			if t == "" {
				t = "Eliminado."
			}
			p.Retirado = t
			posts = append(posts, p)
			continue
		}
		if s := uno(meta, clase("id")); s != nil {
			p.Autor = strings.TrimSpace(strings.TrimPrefix(limpio(texto(s)), "ID"))
		}
		p.Op = uno(meta, clase("marca-op")) != nil
		p.Vos = uno(meta, clase("marca-vos")) != nil
		p.Nuevo = uno(meta, clase("marca-nuevo")) != nil
		p.Sage = uno(meta, clase("sage")) != nil
		if t := uno(meta, tag("time")); t != nil {
			p.Cuando, _ = time.Parse(time.RFC3339Nano, attr(t, "datetime"))
			p.Fecha = normalizarFecha(texto(t))
		}
		if a := uno(meta, clase("reportar")); a != nil {
			href := attr(a, "href")
			p.PuedeBorrar = strings.HasSuffix(href, "/borrar")
			p.PuedeReportar = strings.HasSuffix(href, "/reportar")
		}
		p.Cuerpo = tramos(uno(art, clase("texto")))
		if r := uno(art, clase("respuestas")); r != nil {
			for _, a := range todos(r, tag("a")) {
				if n := num(texto(a)); n > 0 {
					p.Respuestas = append(p.Respuestas, n)
				}
			}
		}
		p.PuedeResponder = uno(art, clase("citar")) != nil
		posts = append(posts, p)
	}
	return posts
}

// ParseHilo lee una publicación (/h/N).
func ParseHilo(doc string, no int) *Hilo {
	raiz := parsear(doc)
	main := uno(raiz, tag("main"))
	h := &Hilo{Cabecera: leerCabecera(raiz), No: no, Aviso: leerAviso(raiz)}
	if h1 := uno(main, tag("h1")); h1 != nil {
		h.Asunto = limpio(texto(h1))
	}
	if arriba := uno(main, id("arriba")); arriba != nil {
		if a := uno(arriba, tag("a")); a != nil {
			if m := regexp.MustCompile(`^/b/([\w-]+)`).FindStringSubmatch(attr(a, "href")); m != nil {
				h.Seccion = m[1]
			}
		}
		h.Fijada = uno(arriba, clase("marca-fijada")) != nil
		for _, b := range todos(arriba, tag("button")) {
			if attr(b, "form") == "guardar" {
				h.PuedeGuardar = true
				h.Guardado = strings.Contains(texto(b), "Sacar")
			}
		}
	}
	for _, p := range todos(main, y(tag("p"), clase("aviso"))) {
		t := limpio(texto(p))
		switch {
		case strings.Contains(t, "archivada"):
			h.Estado, h.EstadoTxt = "archivada", t
		case strings.Contains(t, "cerrada"):
			h.Estado, h.EstadoTxt = "cerrada", t
		}
	}
	lista := uno(main, id("posts"))
	h.Posts = leerPosts(lista)
	if len(h.Posts) > 0 {
		h.Posts[0].Inicial = true
	}
	h.Abierto = lista != nil && tieneAttr(lista, "data-hilo")
	h.Ultimo = num(attr(lista, "data-ultimo"))
	for _, p := range h.Posts {
		if p.No > h.Ultimo {
			h.Ultimo = p.No
		}
	}
	return h
}

// ParseNuevos lee el HTML de /h/N/nuevos.
func ParseNuevos(fragmento string) []Post {
	return leerPosts(parsear("<div>" + fragmento + "</div>"))
}

// ─── Búsqueda ───────────────────────────────────────────────────────────────────────────────

var totalRes = regexp.MustCompile(`^\s*([\d.]+)\s+resultado`)

// ParseBusqueda lee /buscar?q=….
func ParseBusqueda(doc, consulta string) *Busqueda {
	raiz := parsear(doc)
	main := uno(raiz, tag("main"))
	b := &Busqueda{Cabecera: leerCabecera(raiz), Consulta: consulta}
	for _, p := range todos(main, y(tag("p"), clase("ayuda"))) {
		if m := totalRes.FindStringSubmatch(texto(p)); m != nil {
			b.Total, _ = strconv.Atoi(strings.ReplaceAll(m[1], ".", ""))
		}
	}
	if ol := uno(main, y(tag("ol"), clase("resultados"))); ol != nil {
		for _, li := range todos(ol, tag("li")) {
			a := uno(li, clase("res-asunto"))
			if a == nil {
				continue
			}
			r := Resultado{Asunto: limpio(texto(a))}
			if m := regexp.MustCompile(`/h/(\d+)(?:#p(\d+))?`).FindStringSubmatch(attr(a, "href")); m != nil {
				r.Hilo, _ = strconv.Atoi(m[1])
				r.Post, _ = strconv.Atoi(m[2])
			}
			if s := uno(li, y(tag("span"), clase("ayuda"))); s != nil {
				r.Info = normalizarFecha(texto(s))
			}
			r.Fragmento = tramos(uno(li, clase("res-fragmento")))
			b.Resultados = append(b.Resultados, r)
		}
	}
	if b.Total == 0 {
		b.Total = len(b.Resultados)
	}
	b.Pagina, b.Paginas = paginacion(main)
	return b
}

// ─── Cuenta ─────────────────────────────────────────────────────────────────────────────────

var enlaceHilo = regexp.MustCompile(`^/h/(\d+)(?:#p(\d+))?`)

func leerEnlaces(ul *html.Node) []Enlace {
	var out []Enlace
	for _, li := range todos(ul, tag("li")) {
		a := uno(li, tag("a"))
		m := enlaceHilo.FindStringSubmatch(attr(a, "href"))
		if m == nil {
			continue
		}
		e := Enlace{Asunto: limpio(texto(a))}
		e.Hilo, _ = strconv.Atoi(m[1])
		if s := uno(li, clase("ayuda")); s != nil {
			e.Info = strings.TrimSpace(strings.TrimPrefix(normalizarFecha(texto(s)), "·"))
		}
		out = append(out, e)
	}
	return out
}

// ParseBandeja lee /respuestas (al pedirla, el sitio marca los avisos como leídos).
func ParseBandeja(doc string) *Bandeja {
	raiz := parsear(doc)
	main := uno(raiz, tag("main"))
	b := &Bandeja{Cabecera: leerCabecera(raiz)}
	if ul := uno(main, y(tag("ul"), clase("avisos"))); ul != nil {
		for _, li := range todos(ul, tag("li")) {
			a := uno(li, tag("a"))
			m := enlaceHilo.FindStringSubmatch(attr(a, "href"))
			if m == nil {
				continue
			}
			v := Aviso{Asunto: limpio(texto(a)), Nueva: tieneClase(li, "nueva")}
			v.Hilo, _ = strconv.Atoi(m[1])
			v.Post, _ = strconv.Atoi(m[2])
			if s := uno(li, clase("ayuda")); s != nil {
				v.Info = strings.TrimSpace(strings.TrimPrefix(normalizarFecha(texto(s)), "·"))
			}
			if e := uno(li, clase("extracto")); e != nil {
				v.Extracto = limpio(texto(e))
			}
			b.Avisos = append(b.Avisos, v)
		}
	}
	if ul := uno(main, y(tag("ul"), clase("mias"))); ul != nil {
		b.Mias = leerEnlaces(ul)
	}
	return b
}

// ParseGuardados lee /guardados.
func ParseGuardados(doc string) *Guardados {
	raiz := parsear(doc)
	g := &Guardados{Cabecera: leerCabecera(raiz)}
	if ul := uno(uno(raiz, tag("main")), y(tag("ul"), clase("mias"))); ul != nil {
		g.Lista = leerEnlaces(ul)
	}
	return g
}

// ParseMotivos lee el menú de /p/N/reportar.
func ParseMotivos(doc string) []Motivo {
	var out []Motivo
	sel := uno(parsear(doc), y(tag("select"), func(n *html.Node) bool { return attr(n, "name") == "motivo" }))
	for _, o := range todos(sel, tag("option")) {
		out = append(out, Motivo{ID: attr(o, "value"), Titulo: limpio(texto(o))})
	}
	return out
}

// ParseBorrar lee /p/N/borrar: si hay un motivo por el que no se puede, lo devuelve.
func ParseBorrar(doc string) (motivo string) {
	main := uno(parsear(doc), tag("main"))
	if uno(main, y(tag("form"), clase("form-reportar"))) != nil {
		return ""
	}
	if n := uno(main, clase("nota")); n != nil {
		return limpio(texto(n))
	}
	return "No se puede borrar este mensaje."
}

// ParseError devuelve el error que la página muestra (formulario rechazado), o el título.
func ParseError(doc string) string {
	raiz := parsear(doc)
	main := uno(raiz, tag("main"))
	if e := uno(main, clase("error")); e != nil {
		return limpio(texto(e))
	}
	if h1 := uno(main, tag("h1")); h1 != nil {
		t := limpio(texto(h1))
		if p := uno(main, tag("p")); p != nil {
			t += ": " + limpio(texto(p))
		}
		return t
	}
	return ""
}

// ParseDocumento lee una página de texto del sitio (normas, formato, privacidad…).
func ParseDocumento(doc string) *Documento {
	main := uno(parsear(doc), tag("main"))
	d := &Documento{}
	if main == nil {
		return d
	}
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "h1":
				if d.Titulo == "" {
					d.Titulo = limpio(texto(c))
				}
			case "h2":
				d.Bloques = append(d.Bloques, Bloque{Tipo: "h2", Texto: []Tramo{{Texto: limpio(texto(c))}}})
			case "p":
				tipo := "p"
				if tieneClase(c, "ayuda") {
					tipo = "ayuda"
				}
				if ts := tramosDoc(c); len(ts) > 0 {
					d.Bloques = append(d.Bloques, Bloque{Tipo: tipo, Texto: ts})
				}
			case "pre":
				d.Bloques = append(d.Bloques, Bloque{Tipo: "pre", Texto: []Tramo{{Texto: texto(c)}}})
			case "ol", "ul":
				for i, li := range todos(c, tag("li")) {
					d.Bloques = append(d.Bloques, Bloque{Tipo: "li", Texto: tramosDoc(li), Num: i + 1})
				}
			case "div":
				if tieneClase(c, "texto") {
					d.Bloques = append(d.Bloques, Bloque{Tipo: "p", Texto: tramos(c)})
					continue
				}
				rec(c)
			default:
				rec(c)
			}
		}
	}
	rec(main)
	return d
}

// tramosDoc: texto de un párrafo de documento; <strong> y <code> se marcan.
func tramosDoc(n *html.Node) []Tramo {
	var out []Tramo
	var rec func(*html.Node, Estilo)
	rec = func(p *html.Node, e Estilo) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				t := espacios.ReplaceAllString(c.Data, " ")
				if t != "" {
					out = append(out, Tramo{Estilo: e, Texto: t})
				}
			case c.Type != html.ElementNode:
			case c.Data == "br":
				out = append(out, Tramo{Estilo: Salto})
			case c.Data == "strong" || c.Data == "code" || c.Data == "a":
				rec(c, Marca)
			default:
				rec(c, e)
			}
		}
	}
	rec(n, Normal)
	// Recorta espacios en los extremos.
	if len(out) > 0 {
		out[0].Texto = strings.TrimLeft(out[0].Texto, " ")
		out[len(out)-1].Texto = strings.TrimRight(out[len(out)-1].Texto, " ")
	}
	return out
}
