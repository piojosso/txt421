package site

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nombre))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPortadaCatalogo(t *testing.T) {
	l := ParseListado(fixture(t, "portada.html"))
	if len(l.Fichas) != 60 || l.Pagina != 1 || l.Paginas != 12 || l.Vista != "catalogo" {
		t.Fatalf("fichas=%d pagina=%d/%d vista=%s", len(l.Fichas), l.Pagina, l.Paginas, l.Vista)
	}
	f := l.Fichas[0]
	if f.Hilo != 1541 || f.Seccion != "Vida real" || f.R != 19 || !f.Fijada || f.Asunto != "Emu mlydui 5 ojntivbn gehg mvuvpr" {
		t.Fatalf("%+v", f)
	}
	if f.Fecha != "29/9/26, 11:53 p. m." {
		t.Fatalf("fecha %q", f.Fecha)
	}
	if l.Titulo != "" || l.Cabecera.Conectado {
		t.Fatalf("titulo %q conectado %v", l.Titulo, l.Conectado)
	}
	for _, f := range l.Fichas {
		if SeccionPorNombre(f.Seccion) == nil {
			t.Fatalf("sección desconocida %q", f.Seccion)
		}
	}
}

func TestSeccionYArchivo(t *testing.T) {
	l := ParseListado(fixture(t, "seccion.html"))
	if l.Titulo != "Juegos" || l.Paginas != 3 || l.Fichas[0].Seccion != "" || l.Fichas[0].OpNo != 14469 {
		t.Fatalf("%+v %+v", l.Titulo, l.Fichas[0])
	}
	a := ParseListado(fixture(t, "archivo.html"))
	if a.Titulo != "Juegos · archivo" || len(a.Fichas) != 5 || a.Paginas != 1 {
		t.Fatalf("%q %d %d", a.Titulo, len(a.Fichas), a.Paginas)
	}
}

func TestPortadaLista(t *testing.T) {
	l := ParseListado(fixture(t, "portada-lista.html"))
	if l.Vista != "lista" || len(l.Fichas) != 10 {
		t.Fatalf("vista=%s fichas=%d", l.Vista, len(l.Fichas))
	}
	for _, f := range l.Fichas {
		if f.Resumen == nil || f.Resumen.Op.No == 0 || !f.Resumen.Op.Inicial || f.Seccion == "" {
			t.Fatalf("resumen incompleto: %+v", f)
		}
		if len(f.Resumen.Ultimas) > 3 {
			t.Fatalf("más de 3 últimas: %d", len(f.Resumen.Ultimas))
		}
		if f.R > 3 && f.Resumen.Omitidas == 0 {
			t.Fatalf("R=%d sin omitidas", f.R)
		}
	}
}

func TestHiloVisitante(t *testing.T) {
	h := ParseHilo(fixture(t, "hilo.html"), 1542)
	if h.Asunto != "LNIRSL" || h.Seccion != "vida-real" || h.Ultimo != 14433 || !h.Abierto || h.Conectado {
		t.Fatalf("%+v", h)
	}
	if len(h.Posts) != 4 || !h.Posts[0].Inicial || h.Posts[1].Inicial || !h.Posts[2].Op {
		t.Fatalf("posts %+v", h.Posts)
	}
	if h.Posts[1].Respuestas[0] != 14424 || h.Posts[2].Cuerpo[0].Estilo != Ref || h.Posts[2].Cuerpo[0].Num != 14418 {
		t.Fatalf("refs %+v %+v", h.Posts[1].Respuestas, h.Posts[2].Cuerpo)
	}
	if got := TextoPlano(h.Posts[2].Cuerpo); got != ">>14418\nzepgom iz gibl" {
		t.Fatalf("%q", got)
	}
	want := time.Date(2026, 9, 30, 2, 36, 33, 815e6, time.UTC)
	if !h.Posts[0].Cuando.Equal(want) || h.Posts[0].Fecha != "29/9/26, 11:36 p. m." || FechaCorta(want) != h.Posts[0].Fecha {
		t.Fatalf("fecha %v %q %q", h.Posts[0].Cuando, h.Posts[0].Fecha, FechaCorta(want))
	}
	if h.Posts[0].PuedeResponder || h.PuedeGuardar {
		t.Fatal("sin sesión no se puede responder ni guardar")
	}
	if h.Respuestas() != 3 {
		t.Fatal(h.Respuestas())
	}
}

func TestHiloLargo(t *testing.T) {
	h := ParseHilo(fixture(t, "hilo-largo.html"), 1504)
	var retirados int
	var cita bool
	for _, p := range h.Posts {
		if p.Retirado != "" {
			retirados++
			if p.Retirado != "Eliminado por su autor." || p.No == 0 {
				t.Fatalf("%+v", p)
			}
		}
		for _, tr := range p.Cuerpo {
			if tr.Estilo == Cita && tr.Texto == ">sta flievdij d bcatuabuyai" {
				cita = true
			}
		}
	}
	if retirados != 1 || !cita || h.Respuestas() != len(h.Posts)-2 {
		t.Fatalf("retirados=%d cita=%v", retirados, cita)
	}
}

func TestHiloConectado(t *testing.T) {
	h := ParseHilo(fixture(t, "hilo-conectado.html"), 77)
	if !h.Conectado || h.Novedades != 3 || h.CSRF != "tok123" {
		t.Fatalf("cabecera %+v", h.Cabecera)
	}
	if !h.PuedeGuardar || !h.Guardado || !h.Fijada || h.Seccion != "juegos" || h.Aviso == "" {
		t.Fatalf("%+v", h)
	}
	op, r, ret, cola := h.Posts[0], h.Posts[1], h.Posts[2], h.Posts[3]
	if !op.Vos || !op.PuedeBorrar || op.PuedeReportar || !op.PuedeResponder || len(op.Respuestas) != 2 {
		t.Fatalf("op %+v", op)
	}
	if op.Cuerpo[1].Estilo != Spoiler || op.Cuerpo[1].Texto != "secreto" || op.Cuerpo[3].Estilo != Cita {
		t.Fatalf("cuerpo %+v", op.Cuerpo)
	}
	if r.Vos || !r.Nuevo || !r.Sage || !r.PuedeReportar || r.PuedeBorrar {
		t.Fatalf("respuesta %+v", r)
	}
	if ret.Retirado != "Eliminado por moderación." || ret.No != 502 {
		t.Fatalf("retirado %+v", ret)
	}
	if !cola.EnRevision || cola.PuedeResponder || strings.Contains(TextoPlano(cola.Cuerpo), "En revisión") {
		t.Fatalf("en revisión %+v", cola)
	}
}

func TestNuevos(t *testing.T) {
	var d struct{ Html string }
	if err := json.Unmarshal([]byte(fixture(t, "nuevos.json")), &d); err != nil {
		t.Fatal(err)
	}
	ps := ParseNuevos(d.Html)
	if len(ps) != 2 || ps[0].No != 14424 || ps[1].No != 14433 {
		t.Fatalf("%+v", ps)
	}
}

func TestBusqueda(t *testing.T) {
	b := ParseBusqueda(fixture(t, "buscar.html"), "silent")
	if b.Total != 13 || len(b.Resultados) != 13 {
		t.Fatalf("%d %d", b.Total, len(b.Resultados))
	}
	r := b.Resultados[0]
	if r.Hilo != 154 || r.Post != 1488 || r.Info != "Juegos · No.1488 · 24/9/26, 6:52 p. m." {
		t.Fatalf("%+v", r)
	}
	var marca bool
	for _, tr := range r.Fragmento {
		marca = marca || (tr.Estilo == Marca && tr.Texto == "silent")
	}
	if !marca {
		t.Fatalf("%+v", r.Fragmento)
	}
}

func TestBandejaGuardadosMotivos(t *testing.T) {
	b := ParseBandeja(fixture(t, "respuestas.html"))
	if len(b.Avisos) != 2 || !b.Avisos[0].Nueva || b.Avisos[1].Nueva || b.Avisos[0].Post != 501 || b.Avisos[0].Extracto != ">>500 ok" {
		t.Fatalf("%+v", b.Avisos)
	}
	if !strings.HasPrefix(b.Avisos[0].Info, "alguien te respondió") || len(b.Mias) != 1 || b.Mias[0].Hilo != 77 {
		t.Fatalf("%+v %+v", b.Avisos[0], b.Mias)
	}
	g := ParseGuardados(fixture(t, "guardados.html"))
	if len(g.Lista) != 1 || !strings.HasSuffix(g.Lista[0].Info, "archivada") {
		t.Fatalf("%+v", g.Lista)
	}
	m := ParseMotivos(fixture(t, "reportar.html"))
	if len(m) != 3 || m[0] != (Motivo{"respeto", "Sin acoso"}) {
		t.Fatalf("%+v", m)
	}
	if ParseBorrar(fixture(t, "borrar-bloqueado.html")) == "" || ParseBorrar(fixture(t, "reportar.html")) != "" {
		t.Fatal("borrar")
	}
	if ParseError(fixture(t, "error-publicar.html")) != "Esperá 12 segundos antes de publicar otra vez." {
		t.Fatal(ParseError(fixture(t, "error-publicar.html")))
	}
}

func TestDocumentos(t *testing.T) {
	n := ParseDocumento(fixture(t, "normas.html"))
	var items int
	for _, b := range n.Bloques {
		if b.Tipo == "li" {
			items++
		}
	}
	if n.Titulo != "Normas" || items != 8 {
		t.Fatalf("%q %d", n.Titulo, items)
	}
	f := ParseDocumento(fixture(t, "formato.html"))
	var pre, h2 int
	for _, b := range f.Bloques {
		switch b.Tipo {
		case "pre":
			pre++
		case "h2":
			h2++
		}
	}
	if f.Titulo != "Formato" || pre < 3 || h2 < 3 {
		t.Fatalf("%q pre=%d h2=%d", f.Titulo, pre, h2)
	}
}

func TestFormatear(t *testing.T) {
	ts := Formatear(">los videojuegos no son arte\nNo. >>34 y [spoiler]el perro[/spoiler].\n>>12 no es cita")
	want := []Tramo{
		{Estilo: Cita, Texto: ">los videojuegos no son arte"},
		{Estilo: Salto},
		{Estilo: Normal, Texto: "No. "}, {Estilo: Ref, Texto: ">>34", Num: 34}, {Estilo: Normal, Texto: " y "},
		{Estilo: Spoiler, Texto: "el perro"}, {Estilo: Normal, Texto: "."},
		{Estilo: Salto},
		{Estilo: Ref, Texto: ">>12", Num: 12}, {Estilo: Normal, Texto: " no es cita"},
	}
	if len(ts) != len(want) {
		t.Fatalf("%+v", ts)
	}
	for i := range want {
		if ts[i] != want[i] {
			t.Fatalf("%d: %+v != %+v", i, ts[i], want[i])
		}
	}
	// ">>texto" sin número sí es cita (la regla del sitio: /^>(?!>\d)/).
	if Formatear(">>hola")[0].Estilo != Cita || Formatear(">>1")[0].Estilo != Ref {
		t.Fatal("cita vs ref")
	}
}

func TestLimpiarYLargo(t *testing.T) {
	if got := LimpiarTexto("  hola  \r\n\n\n\nchau​ \t"); got != "hola\n\nchau" {
		t.Fatalf("%q", got)
	}
	if Largo("año😀") != 5 {
		t.Fatal(Largo("año😀"))
	}
}

func TestFechas(t *testing.T) {
	if !LeerFecha("29/9/26, 11:53 p. m.").Equal(time.Date(2026, 9, 29, 23, 53, 0, 0, ZonaSitio)) {
		t.Fatal("pm")
	}
	if LeerFecha("1/10/26, 12:05 a. m.").Hour() != 0 || LeerFecha("1/10/26, 12:05 p. m.").Hour() != 12 {
		t.Fatal("12")
	}
	if !LeerFecha("ayer").IsZero() {
		t.Fatal("ayer")
	}
	if FechaCorta(time.Date(2026, 10, 1, 3, 5, 0, 0, time.UTC)) != "1/10/26, 12:05 a. m." {
		t.Fatal(FechaCorta(time.Date(2026, 10, 1, 3, 5, 0, 0, time.UTC)))
	}
}

// ─── Cliente contra un servidor de prueba ───────────────────────────────────────────────────

type falso struct {
	t        *testing.T
	pedidos  []*http.Request
	formas   []url.Values
	responde func(w http.ResponseWriter, r *http.Request)
}

func servidor(t *testing.T, f func(w http.ResponseWriter, r *http.Request)) (*Cliente, *falso) {
	fs := &falso{t: t, responde: f}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		fs.pedidos = append(fs.pedidos, r)
		fs.formas = append(fs.formas, r.PostForm)
		fs.responde(w, r)
	}))
	t.Cleanup(srv.Close)
	c := Nuevo("txt421-test")
	c.Base = srv.URL
	return c, fs
}

func TestClientePublicarYErrores(t *testing.T) {
	c, fs := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/normas":
			http.SetCookie(w, &http.Cookie{Name: "visita", Value: "1.2"})
			w.Write([]byte(fixture(t, "hilo-conectado.html")))
		case r.URL.Path == "/h/77/responder" && r.PostForm.Get("cuerpo") == "mal":
			w.WriteHeader(429)
			w.Write([]byte(fixture(t, "error-publicar.html")))
		case r.URL.Path == "/h/77/responder":
			http.Redirect(w, r, "/h/77?aviso=cola#p504", http.StatusSeeOther)
		case r.URL.Path == "/b/juegos/hilo":
			http.Redirect(w, r, "/h/88", http.StatusSeeOther)
		case r.URL.Path == "/respuestas":
			http.Redirect(w, r, "/entrar", http.StatusSeeOther)
		case r.URL.Path == "/p/5":
			http.Redirect(w, r, "/h/9#p5", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})
	c.SetCookies(map[string]string{"sid": "s3cr3t"})
	ctx := context.Background()

	p, err := c.Responder(ctx, 77, "hola", true)
	if err != nil || p.Hilo != 77 || p.Post != 504 || !p.EnRevision {
		t.Fatalf("%+v %v", p, err)
	}
	// Primero pidió el token (/normas), después publicó con él, sin Origin, con la cookie.
	post := fs.pedidos[1]
	if fs.formas[1].Get("_csrf") != "tok123" || fs.formas[1].Get("sage") != "1" || post.Header.Get("Origin") != "" {
		t.Fatalf("%v %v", fs.formas[1], post.Header)
	}
	if ck, _ := post.Cookie("sid"); ck == nil || ck.Value != "s3cr3t" {
		t.Fatal("sin cookie de sesión")
	}
	if c.Cookies()["visita"] != "1.2" {
		t.Fatal("no guardó visita")
	}

	_, err = c.Responder(ctx, 77, "mal", false)
	var es *ErrSitio
	if !errors.As(err, &es) || es.Status != 429 || !strings.HasPrefix(es.Mensaje, "Esperá 12") {
		t.Fatalf("%v", err)
	}

	p, err = c.Publicar(ctx, "juegos", "asunto", "cuerpo")
	if err != nil || p.Hilo != 88 || p.Post != 0 || p.EnRevision {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := c.Bandeja(ctx); !errors.Is(err, ErrSesion) {
		t.Fatalf("%v", err)
	}
	if _, err := c.Hilo(ctx, 999); !errors.Is(err, ErrNoEncontrado) {
		t.Fatalf("%v", err)
	}
	if h, err := c.Ubicar(ctx, 5); err != nil || h != 9 {
		t.Fatalf("%d %v", h, err)
	}
}

func TestClienteSinSesionNoPublica(t *testing.T) {
	c, fs := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fixture(t, "hilo.html")))
	})
	if _, err := c.Responder(context.Background(), 1542, "hola", false); !errors.Is(err, ErrSesion) {
		t.Fatalf("%v", err)
	}
	for _, r := range fs.pedidos {
		if r.Method == http.MethodPost {
			t.Fatal("no debería haber mandado nada")
		}
	}
}

func TestClienteSinConexion(t *testing.T) {
	c := Nuevo("x")
	c.Base = "http://127.0.0.1:9"
	var er *ErrRed
	if _, err := c.Portada(context.Background(), 1, "catalogo"); !errors.As(err, &er) {
		t.Fatalf("%v", err)
	}
}
