// Package entrar consigue una sesión del sitio: el sitio solo deja entrar con Google, y Google
// solo en un navegador de verdad. Se abre un Chrome/Edge/Brave/Chromium con un perfil propio de
// txt421 y el protocolo de depuración (DevTools) escuchando solo en 127.0.0.1; la persona entra
// como siempre y, apenas el sitio pone la cookie de sesión, se lee y se cierra la ventana.
// El perfil queda guardado, así la próxima vez Google ya recuerda la cuenta.
package entrar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

var (
	// ErrSinNavegador: no hay un navegador compatible instalado.
	ErrSinNavegador = errors.New("no encontré Chrome, Edge, Brave ni Chromium")
	// ErrCerrado: se cerró la ventana antes de entrar.
	ErrCerrado = errors.New("se cerró el navegador antes de entrar")
)

// Navegador encontrado.
type Navegador struct {
	Nombre string
	Ruta   string
}

// candidatos devuelve rutas posibles por sistema, en orden de preferencia.
func candidatos() []Navegador {
	switch runtime.GOOS {
	case "darwin":
		apps := []struct{ nombre, app, bin string }{
			{"Google Chrome", "Google Chrome.app", "Google Chrome"},
			{"Microsoft Edge", "Microsoft Edge.app", "Microsoft Edge"},
			{"Brave", "Brave Browser.app", "Brave Browser"},
			{"Chromium", "Chromium.app", "Chromium"},
			{"Vivaldi", "Vivaldi.app", "Vivaldi"},
		}
		var out []Navegador
		home, _ := os.UserHomeDir()
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			for _, a := range apps {
				out = append(out, Navegador{a.nombre, filepath.Join(base, a.app, "Contents", "MacOS", a.bin)})
			}
		}
		return out
	case "windows":
		var out []Navegador
		bases := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")}
		rel := []struct{ nombre, ruta string }{
			{"Google Chrome", `Google\Chrome\Application\chrome.exe`},
			{"Microsoft Edge", `Microsoft\Edge\Application\msedge.exe`},
			{"Brave", `BraveSoftware\Brave-Browser\Application\brave.exe`},
			{"Chromium", `Chromium\Application\chrome.exe`},
			{"Vivaldi", `Vivaldi\Application\vivaldi.exe`},
		}
		for _, r := range rel {
			for _, b := range bases {
				if b != "" {
					out = append(out, Navegador{r.nombre, filepath.Join(b, r.ruta)})
				}
			}
		}
		return out
	default:
		var out []Navegador
		for _, c := range []struct{ nombre, bin string }{
			{"Google Chrome", "google-chrome-stable"}, {"Google Chrome", "google-chrome"},
			{"Chromium", "chromium"}, {"Chromium", "chromium-browser"},
			{"Brave", "brave-browser"}, {"Brave", "brave"},
			{"Microsoft Edge", "microsoft-edge-stable"}, {"Microsoft Edge", "microsoft-edge"},
			{"Vivaldi", "vivaldi-stable"}, {"Vivaldi", "vivaldi"},
		} {
			if p, err := exec.LookPath(c.bin); err == nil {
				out = append(out, Navegador{c.nombre, p})
			}
		}
		for _, p := range []string{"/usr/bin/chromium", "/snap/bin/chromium", "/var/lib/flatpak/exports/bin/com.google.Chrome"} {
			out = append(out, Navegador{"Chromium", p})
		}
		return out
	}
}

// Buscar devuelve el primer navegador compatible instalado (TXT421_NAVEGADOR lo fuerza).
func Buscar() (Navegador, error) {
	if p := os.Getenv("TXT421_NAVEGADOR"); p != "" {
		return Navegador{filepath.Base(p), p}, nil
	}
	for _, n := range candidatos() {
		if st, err := os.Stat(n.Ruta); err == nil && !st.IsDir() {
			return n, nil
		}
	}
	return Navegador{}, ErrSinNavegador
}

// Resultado de entrar: las cookies del sitio (sid y, si está, visita).
type Resultado struct {
	Cookies   map[string]string
	Navegador string
}

// Entrar abre el navegador en la página de Google del sitio y espera la sesión.
// avisar recibe mensajes de estado para mostrar.
func Entrar(ctx context.Context, base, perfil string, avisar func(string)) (*Resultado, error) {
	nav, err := Buscar()
	if err != nil {
		return nil, err
	}
	host := hostDe(base)
	if err := os.MkdirAll(perfil, 0o700); err != nil {
		return nil, err
	}
	puerto := filepath.Join(perfil, "DevToolsActivePort")
	os.Remove(puerto)

	args := []string{
		"--user-data-dir=" + perfil,
		"--remote-debugging-port=0",
		"--remote-allow-origins=http://127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-default-apps",
		"--new-window",
		"--window-size=520,720",
		"--app=" + base + "/auth/google",
	}
	cmd := exec.Command(nav.Ruta, args...)
	cmd.Stdout, cmd.Stderr = nil, nil
	prepararProceso(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no pude abrir %s: %w", nav.Nombre, err)
	}
	termino := make(chan struct{})
	go func() { cmd.Wait(); close(termino) }()
	defer func() {
		select {
		case <-termino:
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
		}
	}()
	avisar(nav.Nombre)

	// El navegador escribe el puerto elegido en DevToolsActivePort ("puerto\n/devtools/browser/…").
	var wsURL string
	limite := time.Now().Add(30 * time.Second)
	for wsURL == "" {
		if b, err := os.ReadFile(puerto); err == nil {
			lineas := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lineas) >= 2 {
				if p, err := strconv.Atoi(strings.TrimSpace(lineas[0])); err == nil && p > 0 {
					wsURL = fmt.Sprintf("ws://127.0.0.1:%d%s", p, strings.TrimSpace(lineas[1]))
				}
			}
		}
		if wsURL != "" {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-termino:
			return nil, fmt.Errorf("%s se cerró enseguida (¿ya había una ventana de txt421 abierta?)", nav.Nombre)
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(limite) {
			return nil, fmt.Errorf("%s no respondió", nav.Nombre)
		}
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("no pude hablar con %s: %w", nav.Nombre, err)
	}
	conn.SetReadLimit(32 << 20)
	defer conn.CloseNow()

	cdp := &cliente{conn: conn}
	for {
		cookies, err := cdp.cookies(ctx)
		if err != nil {
			select {
			case <-termino:
				return nil, ErrCerrado
			default:
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, ErrCerrado
		}
		encontradas := map[string]string{}
		for _, c := range cookies {
			if strings.TrimPrefix(c.Domain, ".") == host && (c.Name == "sid" || c.Name == "visita") {
				encontradas[c.Name] = c.Value
			}
		}
		if encontradas["sid"] != "" {
			cdp.llamar(ctx, "Browser.close", nil)
			return &Resultado{Cookies: encontradas, Navegador: nav.Nombre}, nil
		}
		select {
		case <-ctx.Done():
			cdp.llamar(context.Background(), "Browser.close", nil)
			return nil, ctx.Err()
		case <-termino:
			return nil, ErrCerrado
		case <-time.After(time.Second):
		}
	}
}

func hostDe(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	return u.Hostname()
}

// cliente mínimo del protocolo DevTools: pedido → respuesta con el mismo id.
type cliente struct {
	conn *websocket.Conn
	id   int
}

type cookieCDP struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

func (c *cliente) llamar(ctx context.Context, metodo string, params any) (json.RawMessage, error) {
	c.id++
	pedido := map[string]any{"id": c.id, "method": metodo}
	if params != nil {
		pedido["params"] = params
	}
	b, _ := json.Marshal(pedido)
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.conn.Write(cctx, websocket.MessageText, b); err != nil {
		return nil, err
	}
	for {
		_, datos, err := c.conn.Read(cctx)
		if err != nil {
			return nil, err
		}
		var r struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(datos, &r) != nil || r.ID != c.id {
			continue // eventos u otras respuestas
		}
		if r.Error != nil {
			return nil, errors.New(r.Error.Message)
		}
		return r.Result, nil
	}
}

func (c *cliente) cookies(ctx context.Context) ([]cookieCDP, error) {
	res, err := c.llamar(ctx, "Storage.getCookies", nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		Cookies []cookieCDP `json:"cookies"`
	}
	err = json.Unmarshal(res, &r)
	return r.Cookies, err
}
