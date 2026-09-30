// txt421: cliente de terminal para txt.421.news, el foro de texto de 421.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // las fechas del sitio van en hora de Argentina, también en Windows

	tea "charm.land/bubbletea/v2"

	"github.com/piojosso/txt421/internal/actualizar"
	"github.com/piojosso/txt421/internal/config"
	"github.com/piojosso/txt421/internal/i18n"
	"github.com/piojosso/txt421/internal/site"
	"github.com/piojosso/txt421/internal/tui"
)

// version la pone el armado de los releases (-ldflags "-X main.version=…").
var version = "dev"

const ayuda = `txt421 — cliente de terminal para txt.421.news, el foro de texto de 421.

Uso:
  txt421                   abre la portada
  txt421 1542              abre la publicación 1542 (vale pegar el link)
  txt421 b juegos          abre una sección (tecnologia, cultura, musica, juegos, vida-real)
  txt421 buscar <texto>    busca en el foro
  txt421 respuestas        tus respuestas (hace falta entrar)
  txt421 guardados         tus publicaciones guardadas
  txt421 entrar            entrar con Google
  txt421 salir             cerrar la sesión
  txt421 actualizar        bajar e instalar la última versión
  txt421 version

Adentro: flechas para moverse, Enter abre, Esc vuelve, ? muestra todas las teclas.
Código y reportes: https://github.com/piojosso/txt421
`

func main() {
	actualizar.Limpiar()
	i18n.Usar(config.CargarPreferencias().Idioma)
	args := os.Args[1:]
	abrir := ""
	if len(args) > 0 {
		switch a := strings.ToLower(args[0]); {
		case a == "-h" || a == "--help" || a == "help" || a == "ayuda":
			fmt.Print(ayuda)
			return
		case a == "-v" || a == "--version" || a == "version" || a == "versión":
			fmt.Println("txt421", version)
			return
		case a == "actualizar" || a == "update":
			os.Exit(cmdActualizar())
		case a == "salir" || a == "logout":
			os.Exit(cmdSalir())
		case a == "entrar" || a == "login":
			abrir = "entrar"
		case a == "respuestas" || a == "guardados":
			abrir = a
		case (a == "b" || a == "seccion" || a == "sección") && len(args) > 1:
			abrir = "b/" + slug(args[1])
		case a == "buscar" || a == "search" || a == "/":
			abrir = "buscar/" + strings.Join(args[1:], " ")
		case (a == "p" || a == "post") && len(args) > 1:
			abrir = "p/" + strings.TrimLeft(args[1], ">")
		default:
			if ref := refHilo(strings.Join(args, " ")); ref != "" {
				abrir = "h/" + ref
			} else {
				fmt.Fprintf(os.Stderr, "txt421: no entiendo %q. Probá txt421 --help\n", args[0])
				os.Exit(2)
			}
		}
	}

	op := tui.Opciones{
		Version:   version,
		UserAgent: "txt421/" + version + " (+https://github.com/" + actualizar.Repo + ")",
		Abrir:     abrir,
		Actualizar: func(ctx context.Context) (string, error) {
			return actualizar.Actualizar(ctx, version)
		},
		UltimaVersion: actualizar.Ultima,
	}
	if version == "dev" {
		op.Actualizar = nil
		op.UltimaVersion = nil
	}
	p := tea.NewProgram(tui.Nueva(op))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "txt421:", err)
		os.Exit(1)
	}
}

var reHilo = regexp.MustCompile(`(?:^|/)h/(\d+)(?:\.txt)?(?:[^#]*#p(\d+))?|^#?(\d+)(?:#p(\d+))?$`)

// refHilo: "1542", "#1542", ".../h/1542#p14418" → "1542" o "1542#p14418".
func refHilo(s string) string {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		s = u.Path
		if u.Fragment != "" {
			s += "#" + u.Fragment
		}
	}
	m := reHilo.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	no, post := m[1], m[2]
	if no == "" {
		no, post = m[3], m[4]
	}
	if post != "" {
		return no + "#p" + post
	}
	return no
}

func slug(s string) string {
	s = strings.ToLower(strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", " ", "-").Replace(s))
	for _, sec := range site.Secciones {
		if sec.Slug == s || strings.HasPrefix(sec.Slug, s) {
			return sec.Slug
		}
	}
	return s
}

func cmdActualizar() int {
	if version == "dev" {
		fmt.Println("txt421: esta es una versión de desarrollo; no se actualiza sola.")
		return 1
	}
	fmt.Println("Buscando la última versión…")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	v, err := actualizar.Actualizar(ctx, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "txt421:", err)
		return 1
	}
	if v == version {
		fmt.Println("Ya tenés la última versión:", v)
	} else {
		fmt.Println("Listo: txt421", v)
	}
	return 0
}

func cmdSalir() int {
	c := site.Nuevo("txt421/" + version)
	c.SetCookies(config.CargarCookies())
	if c.Sesion() == "" {
		fmt.Println("No habías entrado.")
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := c.Salir(ctx)
	cookies := c.Cookies()
	delete(cookies, "sid")
	config.GuardarCookies(cookies)
	if err != nil {
		fmt.Fprintln(os.Stderr, "txt421: la sesión se olvidó acá, pero el sitio no respondió:", err)
		return 1
	}
	fmt.Println("Saliste.")
	return 0
}
