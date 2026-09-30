package tui

import (
	"image/color"
	"strings"

	. "github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// ─── Ficha del catálogo (.ficha) ────────────────────────────────────────────────────────────

const (
	anchoFichaMin = 26 // minmax(14.5rem, 1fr)
	altoFicha     = 9
)

// dibujarFicha: borde, barra (sección · marcas · R), línea punteada, asunto, texto y fecha.
func (a *App) dibujarFicha(f site.Ficha, w, alto int, sel bool) []Linea {
	alto = max(alto, altoFicha)
	t := a.tema
	bg := t.Caja
	b := bordeSimple
	eb := Est{Fg: t.Borde, Bg: t.Fondo}
	if f.Fijada {
		bg = a.es.fijada
		b = bordeGrueso
		eb = Est{Fg: t.Aviso, Bg: t.Fondo}
	}
	if sel {
		eb = Est{Fg: t.Fosforo, Bg: t.Fondo}
		if f.Fijada {
			b = bordeGrueso
		}
	}
	base := Est{Bg: bg}
	in := w - 2 // interior
	pad := 1
	iw := in - 2*pad

	// Barra: a la izquierda la sección (o el No.), a la derecha Fijada, nuevo y R.
	tenueBarra := base.conFg(t.Tenue)
	if sel {
		tenueBarra = base.conFg(t.Fosforo)
	}
	izq := f.Seccion
	if izq == "" && f.OpNo > 0 {
		izq = "No." + itoa(f.OpNo)
	}
	var der Linea
	if f.Fijada {
		der = append(der, Run{Est{Fg: t.Fondo, Bg: t.Aviso, Negrita: true}, " " + mayus(T("fijada")) + " "}, Run{base, " "})
	}
	if f.Novedad != "" {
		der = append(der, Run{base.conFg(t.Fosforo).negrita(), mayus(f.Novedad)}, Run{base, " "})
	}
	der = append(der, Run{tenueBarra, "R: " + itoa(f.R)})
	izqL := txt(tenueBarra, recortar(mayus(izq), max(1, iw-der.Ancho()-1)))
	barra := aDerecha(izqL, der, iw, base)

	sepColor := t.Borde
	if f.Fijada {
		sepColor = t.Aviso
	}
	if sel {
		sepColor = t.Fosforo
	}
	sep := txt(base.conFg(sepColor), strings.Repeat("┄", iw))

	titulo := envolver([]site.Tramo{{Texto: f.Asunto}}, iw, estilosTexto{normal: base.conFg(t.Fosforo).negrita()}, false)
	titulo = cortarConElipsis(titulo, 2, iw)
	// Lo que queda entre el título y la fecha es para el texto (como el flex: 1 de .ficha-texto).
	cuerpoLineas := max(1, alto-2-3-len(titulo))
	texto := envolver([]site.Tramo{{Texto: f.Extracto}}, iw, estilosTexto{normal: base.conFg(t.Texto)}, false)
	texto = cortarConElipsis(texto, cuerpoLineas, iw)

	pie := f.Fecha
	if f.Cerrada {
		pie += " · " + T("cerrada")
	}
	interior := []Linea{barra, sep}
	interior = append(interior, titulo...)
	interior = append(interior, texto...)
	for len(interior) < alto-3 {
		interior = append(interior, nil)
	}
	interior = append(interior, txt(base.conFg(t.Tenue), recortar(pie, iw)))
	for i := range interior {
		interior[i] = mas(Linea{espacio(base, pad)}, completar(interior[i], iw, base), Linea{espacio(base, pad)})
	}
	return caja(interior, w, b, eb, base)
}

// cortarConElipsis deja n líneas y, si sobraba texto, termina la última con "…".
func cortarConElipsis(ls []Linea, n, w int) []Linea {
	if len(ls) <= n {
		return ls
	}
	ls = ls[:n]
	u := ls[n-1]
	if u.Ancho() >= w {
		u = cortarLinea(u, 0, w-1)
	}
	if len(u) > 0 {
		u = append(u, Run{u[len(u)-1].E, "…"})
	}
	ls[n-1] = u
	return ls
}

// ─── Mensaje (.post) ────────────────────────────────────────────────────────────────────────

// opcionesPost para dibujar un mensaje.
type opcionesPost struct {
	sel      bool // seleccionado (:target en el sitio)
	destapar bool // spoilers visibles (el sitio los muestra al pasar el mouse o con foco)
	maxTexto int  // líneas de texto como mucho (0 = todas); para listados
	acciones bool // mostrar "Responder" (solo en el seleccionado)
}

// sangriaRespuesta: .post.respuesta { margin-left: 1.75rem }.
const sangriaRespuesta = 3

// dibujarPost devuelve las líneas del mensaje, ya con la sangría de las respuestas.
func (a *App) dibujarPost(p site.Post, w int, o opcionesPost) []Linea {
	t := a.tema
	sangria := 0
	if !p.Inicial {
		sangria = sangriaRespuesta
		if w < 50 {
			sangria = 1
		}
	}
	bw := w - sangria
	margen := Linea{espacio(a.es.fondo, sangria)}
	iw := bw - 4 // bordes y un espacio de cada lado

	// Retirado: caja punteada, transparente, una línea.
	if p.Retirado != "" {
		e := a.es.tenue
		l := txt(e, recortar("No."+itoa(p.No)+" · "+p.Retirado, iw))
		out := caja([]Linea{mas(Linea{espacio(a.es.fondo, 1)}, completar(l, iw, a.es.fondo), Linea{espacio(a.es.fondo, 1)})}, bw, bordePunteado, a.es.borde, a.es.fondo)
		// Sin bordes arriba y abajo: .retirado tiene poco padding.
		out = out[1:2]
		out[0] = mas(txt(a.es.borde, "┆"), cortarLinea(out[0], 1, bw-1), txt(a.es.borde, "┆"))
		return conMargen(out, margen)
	}

	bg := t.Caja
	if o.sel {
		bg = a.es.sel
	}
	base := Est{Bg: bg}
	b := bordeSimple
	colorBorde := t.Borde
	if p.Inicial {
		colorBorde = t.BordeOp
	}
	if p.EnRevision {
		b = bordePunteado
		colorBorde = t.Aviso
	}
	if o.sel {
		colorBorde = t.Fosforo
	}
	eb := Est{Fg: colorBorde, Bg: t.Fondo}

	// Datos: ID, OP, (vos), nuevo, fecha … No.N, sage, Reportar/Borrar.
	barraLlena := p.Inicial && t.BarraOpLlena && !p.EnRevision
	meta := base.conFg(t.Tenue)
	num := base.conFg(t.Fosforo).negrita()
	op := Est{Fg: t.Fondo, Bg: t.Fosforo, Negrita: true}
	nuevo := base.conFg(t.Fosforo).negrita()
	if p.EnRevision {
		meta = base.conFg(t.Aviso)
	}
	if barraLlena {
		fondoBarra := t.Fosforo
		meta = Est{Fg: t.Fondo, Bg: fondoBarra, Negrita: true}
		num = meta
		op = Est{Fg: t.Fosforo, Bg: t.Fondo, Negrita: true}
		nuevo = meta
	}
	may := func(s string) string {
		if barraLlena {
			return mayus(s)
		}
		return s
	}
	var izq Linea
	sepMeta := Run{meta, "  "}
	izq = append(izq, Run{meta, may("ID " + p.Autor)})
	if p.Op {
		izq = append(izq, sepMeta, Run{op, " OP "})
	}
	if p.Vos {
		izq = append(izq, sepMeta, Run{meta, may(T("vos"))})
	}
	if p.Nuevo {
		izq = append(izq, sepMeta, Run{nuevo, mayus(T("nuevo"))})
	}
	fecha := p.Fecha
	if fecha == "" && !p.Cuando.IsZero() {
		fecha = site.FechaCorta(p.Cuando)
	}
	izq = append(izq, sepMeta, Run{meta, may(fecha)})
	var der Linea
	der = append(der, Run{num, may("No." + itoa(p.No))})
	if p.Sage {
		der = append(der, Run{meta, " " + may("sage")})
	}
	if p.PuedeBorrar {
		der = append(der, Run{meta, "  " + may(T("borrar"))})
	} else if p.PuedeReportar {
		der = append(der, Run{meta, "  " + may(T("reportar"))})
	}
	fondoMeta := base
	if barraLlena {
		fondoMeta = Est{Bg: t.Fosforo}
	}
	var metaL Linea
	if izq.Ancho()+der.Ancho()+1 > iw {
		// No entra en una línea: la fecha va con el número.
		metaL = aDerecha(izq[:min(len(izq), 1)], der, iw, fondoMeta)
		if p.Op {
			metaL = aDerecha(mas(Linea{izq[0]}, Linea{sepMeta, Run{op, " OP "}}), der, iw, fondoMeta)
		}
	} else {
		metaL = aDerecha(izq, der, iw, fondoMeta)
	}

	var interior []Linea
	if p.EnRevision {
		interior = append(interior, a.parrafo(T("en_revision"), iw, base.conFg(t.Aviso))...)
	}
	texto := envolver(p.Cuerpo, iw, a.textoSobre(bg), o.destapar)
	if o.maxTexto > 0 && len(texto) > o.maxTexto {
		texto = cortarConElipsis(texto, o.maxTexto, iw)
	}
	interior = append(interior, texto...)
	if len(p.Respuestas) > 0 && o.maxTexto == 0 {
		refs := Linea{{base.conFg(t.Tenue), T("respuestas_de") + " "}}
		for i, n := range p.Respuestas {
			if i > 0 {
				refs = append(refs, Run{base, " "})
			}
			refs = append(refs, Run{base.conFg(t.Cita).subrayado(), ">>" + itoa(n)})
		}
		interior = append(interior, envolverLinea(refs, iw)...)
	}
	if o.acciones && p.PuedeResponder {
		interior = append(interior, txt(base.conFg(t.Fosforo).negrita(), T("responder")))
	}

	pad := func(l Linea, e Est) Linea {
		return mas(Linea{espacio(e, 1)}, completar(l, iw, e), Linea{espacio(e, 1)})
	}
	var out []Linea
	if barraLlena {
		// La "ventana": barra de título llena arriba, sin línea de borde.
		barraEst := Est{Bg: t.Fosforo}
		out = append(out, mas(txt(Est{Fg: t.Fosforo, Bg: t.Fosforo}, "█"), pad(metaL, barraEst), txt(Est{Fg: t.Fosforo, Bg: t.Fosforo}, "█")))
		for _, l := range interior {
			out = append(out, mas(txt(eb, b.i), pad(l, base), txt(eb, b.d)))
		}
		out = append(out, txt(eb, b.ii+strings.Repeat(b.s, bw-2)+b.id))
		if o.sel {
			// Seleccionado: la barra sigue llena; el borde de fósforo ya marca.
		}
	} else {
		filas := []Linea{pad(metaL, base)}
		if p.Inicial && !t.BarraOpLlena {
			// descanso / monocromo: barra con el fondo de la caja y una línea abajo.
			filas = append(filas, pad(txt(base.conFg(t.Borde), strings.Repeat("─", iw)), base))
		}
		for _, l := range interior {
			filas = append(filas, pad(l, base))
		}
		out = caja(filas, bw, b, eb, base)
	}
	return conMargen(out, margen)
}

func conMargen(ls []Linea, margen Linea) []Linea {
	if margen.Ancho() == 0 {
		return ls
	}
	for i := range ls {
		ls[i] = mas(margen, ls[i])
	}
	return ls
}

// envolverLinea parte una línea ya armada en renglones de w columnas (por pedazos).
func envolverLinea(l Linea, w int) []Linea {
	var out []Linea
	var actual Linea
	for _, r := range l {
		if actual.Ancho()+ancho(r.T) > w && len(actual) > 0 {
			out = append(out, actual)
			actual = nil
			if strings.TrimSpace(r.T) == "" {
				continue
			}
		}
		actual = append(actual, r)
	}
	if len(actual) > 0 {
		out = append(out, actual)
	}
	return out
}

// colorTexto devuelve el color del texto normal (para algunos armados).
func (a *App) colorTexto() color.Color { return a.tema.Texto }
