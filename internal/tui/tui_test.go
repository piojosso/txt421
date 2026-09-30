package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
)

// sitioFalso sirve las páginas guardadas en internal/site/testdata.
func sitioFalso(t *testing.T, conectado bool) *httptest.Server {
	t.Helper()
	leer := func(n string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "site", "testdata", n))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n string
		switch {
		case r.URL.Path == "/" && r.URL.Query().Get("vista") == "lista":
			n = "portada-lista.html"
		case r.URL.Path == "/":
			n = "portada.html"
		case r.URL.Path == "/b/juegos":
			n = "seccion.html"
		case r.URL.Path == "/b/juegos/archivo":
			n = "archivo.html"
		case r.URL.Path == "/h/1542":
			n = "hilo.html"
		case r.URL.Path == "/h/1504":
			n = "hilo-largo.html"
		case r.URL.Path == "/h/77":
			n = "hilo-conectado.html"
		case r.URL.Path == "/buscar":
			n = "buscar.html"
		case r.URL.Path == "/normas":
			if conectado {
				n = "hilo-conectado.html"
			} else {
				n = "normas.html"
			}
		case r.URL.Path == "/formato":
			n = "formato.html"
		case r.URL.Path == "/respuestas":
			n = "respuestas.html"
		case r.URL.Path == "/guardados":
			n = "guardados.html"
		case r.URL.Path == "/p/501/reportar":
			n = "reportar.html"
		case strings.HasSuffix(r.URL.Path, "/nuevos"):
			w.Write([]byte(`{"ultimo":0,"html":""}`))
			return
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(leer(n))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// correr ejecuta un comando y los que devuelva (sin esperar ticks largos).
func correr(a *App, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(400 * time.Millisecond):
		return // un tick: se ignora
	}
	switch m := msg.(type) {
	case nil, tickCursor, tickEnvio, vivoTick:
	case tea.BatchMsg:
		for _, c := range m {
			correr(a, c)
		}
	default:
		if reflect.TypeOf(msg).Kind() == reflect.Slice {
			return
		}
		_, c := a.Update(msg)
		correr(a, c)
	}
}

type prueba struct {
	t           *testing.T
	a           *App
	ancho, alto int
}

func nuevaPrueba(t *testing.T, ancho, alto int, conectado bool, abrir string) *prueba {
	t.Setenv("TXT421_CONFIG", t.TempDir())
	t.Setenv("TXT421_LANG", "es")
	i18n.Usar("es")
	srv := sitioFalso(t, conectado)
	a := Nueva(Opciones{Version: "dev", UserAgent: "prueba", Abrir: abrir})
	a.cliente.Base = srv.URL
	if conectado {
		a.cliente.SetCookies(map[string]string{"sid": "x"})
	}
	a.Update(tea.WindowSizeMsg{Width: ancho, Height: alto})
	correr(a, a.Init())
	return &prueba{t, a, ancho, alto}
}

func (p *prueba) tecla(teclas ...string) {
	for _, k := range teclas {
		var m tea.KeyPressMsg
		switch k {
		case "enter":
			m = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			m = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "down":
			m = tea.KeyPressMsg{Code: tea.KeyDown}
		case "up":
			m = tea.KeyPressMsg{Code: tea.KeyUp}
		case "right":
			m = tea.KeyPressMsg{Code: tea.KeyRight}
		case "left":
			m = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "tab":
			m = tea.KeyPressMsg{Code: tea.KeyTab}
		case "pgdown":
			m = tea.KeyPressMsg{Code: tea.KeyPgDown}
		default:
			r := []rune(k)
			m = tea.KeyPressMsg{Code: r[0], Text: k}
		}
		_, c := p.a.Update(m)
		correr(p.a, c)
	}
}

// pantalla dibuja y controla que cada línea tenga el ancho justo; con TXT421_SNAP guarda el ANSI.
func (p *prueba) pantalla(nombre string) string {
	p.t.Helper()
	v := p.a.View()
	lineas := strings.Split(v.Content, "\n")
	if len(lineas) != p.alto {
		p.t.Fatalf("%s: %d líneas, quería %d", nombre, len(lineas), p.alto)
	}
	for i, l := range lineas {
		if w := ansi.StringWidth(l); w != p.ancho {
			p.t.Fatalf("%s: línea %d mide %d, quería %d: %q", nombre, i, w, p.ancho, ansi.Strip(l))
		}
	}
	if dir := os.Getenv("TXT421_SNAP"); dir != "" {
		os.WriteFile(filepath.Join(dir, nombre+".ansi"), []byte(v.Content), 0o644)
	}
	return ansi.Strip(v.Content)
}

func contiene(t *testing.T, pantalla string, textos ...string) {
	t.Helper()
	for _, s := range textos {
		if !strings.Contains(pantalla, s) {
			t.Fatalf("falta %q en:\n%s", s, pantalla)
		}
	}
}

func TestPortadaCatalogo(t *testing.T) {
	p := nuevaPrueba(t, 120, 40, false, "")
	s := p.pantalla("portada")
	contiene(t, s, ">_ TXT", "TECNOLOGÍA", "VIDA REAL", "ENTRAR", "Emu mlydui 5 ojntivbn", "R: 19", "FIJADA", "CATÁLOGO", "Entrá para publicar")
	// Flechas: la selección se mueve y pasa de página al llegar al final.
	lp := p.a.actual().(*pantListado)
	p.tecla("right", "down")
	if lp.sel != 1+lp.cols {
		t.Fatalf("sel=%d cols=%d", lp.sel, lp.cols)
	}
	p.tecla("pgdown")
	if lp.sel/lp.porPagina != 1 {
		t.Fatalf("sel=%d porPagina=%d", lp.sel, lp.porPagina)
	}
	s = p.pantalla("portada-p2")
	contiene(t, s, "Siguiente →", "← Anterior")
}

func TestPortadaChica(t *testing.T) {
	p := nuevaPrueba(t, 60, 18, false, "")
	p.pantalla("portada-chica")
	p2 := nuevaPrueba(t, 40, 10, false, "")
	contiene(t, p2.pantalla("muy-chica"), "Agrandá la ventana")
}

func TestVistaLista(t *testing.T) {
	p := nuevaPrueba(t, 100, 45, false, "")
	p.tecla("v")
	s := p.pantalla("portada-lista")
	contiene(t, s, "LISTA", "ID ")
	p.tecla("right")
	p.pantalla("portada-lista-p2")
}

func TestSeccion(t *testing.T) {
	p := nuevaPrueba(t, 100, 40, false, "b/juegos")
	s := p.pantalla("seccion")
	contiene(t, s, "> Juegos", "Videojuegos, juegos de mesa", "Archivo", "NO.14469")
	p.tecla("a")
	contiene(t, p.pantalla("archivo"), "Juegos · archivo", "Volver a las publicaciones activas")
}

func TestHilo(t *testing.T) {
	p := nuevaPrueba(t, 100, 40, false, "h/1542")
	s := p.pantalla("hilo")
	contiene(t, s, "> LNIRSL", "← Vida real", "ID 8PWWBMK8", "NO.14411", "No.14418", "Respuestas: >>14424", "Entrá para responder")
	h := p.a.actual().(*pantHilo)
	p.tecla("down", "down")
	if h.sel != 2 {
		t.Fatal(h.sel)
	}
	p.tecla("i") // 14424 cita a 14418
	if h.hilo.Posts[h.sel].No != 14418 {
		t.Fatalf("i fue a %d", h.hilo.Posts[h.sel].No)
	}
	p.tecla("esc")
	if h.hilo.Posts[h.sel].No != 14424 {
		t.Fatalf("esc volvió a %d", h.hilo.Posts[h.sel].No)
	}
	p.tecla("esc")
	if _, ok := p.a.actual().(*pantListado); !ok {
		t.Fatal("esc no volvió a la portada")
	}
}

func TestHiloLargoEnPaginas(t *testing.T) {
	p := nuevaPrueba(t, 90, 30, false, "h/1504")
	h := p.a.actual().(*pantHilo)
	p.pantalla("hilo-largo")
	if len(h.paginas) < 5 {
		t.Fatalf("páginas=%d", len(h.paginas))
	}
	var todo strings.Builder
	for i := 0; i < len(h.paginas)+2; i++ {
		todo.WriteString(p.pantalla("hilo-largo-p"))
		p.tecla("right")
	}
	if h.pag != len(h.paginas)-1 {
		t.Fatalf("pag=%d de %d", h.pag, len(h.paginas))
	}
	contiene(t, todo.String(), "Eliminado por su autor", "sta flievdij d bcatuabuyai")
	// Todos los mensajes aparecen en alguna página.
	for _, post := range h.hilo.Posts {
		if !strings.Contains(todo.String(), itoa(post.No)) {
			t.Fatalf("falta el No.%d", post.No)
		}
	}
}

func TestHiloConectadoYComposer(t *testing.T) {
	p := nuevaPrueba(t, 100, 40, true, "h/77")
	s := p.pantalla("hilo-conectado")
	contiene(t, s, "RESPUESTAS 3", "SACAR DE GUARDADOS", "(VOS)", "BORRAR", "Reportar", "sage", "En revisión", "RESPONDER", "Eliminado por moderación")
	h := p.a.actual().(*pantHilo)
	if h.elegido().No != 501 {
		t.Fatalf("arrancó en %d, no en el primero nuevo", h.elegido().No)
	}
	p.tecla("enter")
	c, ok := p.a.actual().(*pantComposer)
	if !ok {
		t.Fatalf("enter no abrió el formulario: %T", p.a.actual())
	}
	if c.area.Value() != ">>501\n" {
		t.Fatalf("%q", c.area.Value())
	}
	p.tecla("h", "o", "l", "a")
	s = p.pantalla("composer")
	contiene(t, s, "> Responder", "En: Hilo de prueba", "[ PUBLICAR ]", "[ VISTA PREVIA ]", "sage: responder sin subir", "/ 8.000")
	p.tecla("tab", "tab", "enter") // Vista previa
	contiene(t, p.pantalla("composer-previa"), "Vista previa: todavía no se publicó.", "[ EDITAR ]")
	// Esc guarda el borrador; al volver a responder se recupera.
	p.tecla("esc")
	if _, ok := p.a.actual().(*pantHilo); !ok {
		t.Fatal("esc no volvió")
	}
	if b := p.a.borradores["respuesta:77"]; !strings.Contains(b.Cuerpo, "hola") {
		t.Fatalf("borrador %+v", b)
	}
	p.tecla("r")
	if c := p.a.actual().(*pantComposer); !strings.Contains(c.area.Value(), "hola") {
		t.Fatal("no recuperó el borrador")
	}
	_ = h
}

func TestReportarYMenu(t *testing.T) {
	p := nuevaPrueba(t, 100, 40, true, "h/77")
	p.tecla("x") // arranca en 501, el primero nuevo
	if _, ok := p.a.capa.(*dialogoReportar); !ok {
		t.Fatalf("capa %T", p.a.capa)
	}
	contiene(t, p.pantalla("reportar"), "> REPORTAR", "(•) Sin acoso", "( ) Sin spam", "[ ENVIAR REPORTE ]")
	p.tecla("esc")
	p.tecla("m")
	contiene(t, p.pantalla("menu"), "Respuestas (3)", "Guardados", "Preferencias", "Cerrar txt421")
	p.tecla("esc", "?")
	contiene(t, p.pantalla("ayuda"), "> TECLAS")
}

func TestBuscarRespuestasDocs(t *testing.T) {
	p := nuevaPrueba(t, 100, 40, false, "buscar/silent")
	contiene(t, p.pantalla("buscar"), "> Buscar", "13 resultados, incluido el archivo.", "tpy vnt rzgfj gavr pt aj OT5?", "Juegos · No.1488")
	p2 := nuevaPrueba(t, 100, 40, true, "respuestas")
	contiene(t, p2.pantalla("respuestas"), "> Respuestas", "Hilo de prueba", "alguien te respondió", "Donde participaste")
	p3 := nuevaPrueba(t, 100, 40, false, "")
	p3.a.Ir(nuevaDocumento("/normas", "Normas"))
	correr(p3.a, p3.a.actual().Recargar(p3.a))
	contiene(t, p3.pantalla("normas"), "> Normas", "1. Sin acoso.")
	p3.a.Ir(nuevaEntrar())
	contiene(t, p3.pantalla("entrar"), "> Entrar", "[ ENTRAR CON GOOGLE ]", "PEGAR LA COOKIE")
}

func TestTemasSinRomper(t *testing.T) {
	for _, tema := range Temas {
		p := nuevaPrueba(t, 100, 40, true, "h/77")
		p.a.usarTema(tema.Nombre)
		p.pantalla("tema-" + tema.Nombre)
	}
}

func TestEnvolver(t *testing.T) {
	es := estilosTexto{}
	for _, w := range []int{2, 3, 7, 30} {
		ls := envolver([]site.Tramo{{Texto: "palabra " + strings.Repeat("x", 45) + " 日本語テキスト fin"}}, w, es, false)
		for _, l := range ls {
			if l.Ancho() > w {
				t.Fatalf("w=%d: %v mide %d", w, l, l.Ancho())
			}
		}
	}
}
