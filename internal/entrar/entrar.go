// Package entrar consigue una sesión del sitio: el sitio solo deja entrar con Google, y Google
// solo en un navegador de verdad y sin automatizar. Se usa un Chrome/Edge/Brave/Chromium con un
// perfil propio de txt421: la persona entra ahí como siempre, cierra la ventana, y después se lee
// la cookie de sesión abriendo ese perfil sin ventana (ver Entrar). El perfil queda guardado, así
// la próxima vez Google ya recuerda la cuenta.
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
	"sync"
	"time"

	"github.com/coder/websocket"
)

var (
	// ErrSinNavegador: no hay un navegador compatible instalado.
	ErrSinNavegador = errors.New("no encontré Chrome, Edge, Brave ni Chromium")
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
		for _, p := range []string{"/usr/bin/chromium", "/snap/bin/chromium"} {
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

// ErrNoEntro: se cerró el navegador sin que el sitio haya dado una sesión.
var ErrNoEntro = errors.New("no se entró: el navegador se cerró sin sesión del sitio")

// ErrYaAbierto: ya había una ventana de txt421 abierta con ese perfil.
var ErrYaAbierto = errors.New("ya hay una ventana de txt421 abierta: cerrala y probá de nuevo")

// Entrar hace el login en tres pasos. Google no deja entrar en un navegador con el protocolo de
// depuración abierto ("este navegador puede no ser seguro"), así que:
//  1. se abre el perfil sin ventana (headless) con DevTools y se borra la sesión vieja del sitio;
//  2. se abre el navegador normal, sin DevTools, en la página de Google del sitio, y se espera a
//     que la persona lo cierre (o a que avise por listo);
//  3. se vuelve a abrir el perfil sin ventana y se lee la cookie de sesión que dejó el sitio.
//
// avisar recibe el nombre del navegador cuando la ventana ya está abierta.
func Entrar(ctx context.Context, base, perfil string, avisar func(string), listo <-chan struct{}) (*Resultado, error) {
	nav, err := Buscar()
	if err != nil {
		return nil, err
	}
	perfil = perfilPara(nav, perfil)
	if err := os.MkdirAll(perfil, 0o700); err != nil {
		return nil, err
	}
	activos.Add(1)
	defer activos.Done()

	// 1. Sin la sesión vieja (si quedó una, se la leería al final aunque ya no sirva).
	if _, err := conDevTools(ctx, nav, perfil, func(c *cliente) (map[string]string, error) {
		return nil, c.borrarSesion(ctx, base)
	}); err != nil {
		return nil, fmt.Errorf("no pude preparar %s: %w", nav.Nombre, err)
	}

	// 2. El navegador de verdad, sin nada raro: Google lo trata como a cualquiera.
	cmd := exec.Command(nav.Ruta, append(banderas(perfil),
		"--new-window",
		"--window-size=560,760",
		"--app="+base+"/auth/google",
	)...)
	prepararProceso(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no pude abrir %s: %w", nav.Nombre, err)
	}
	inicio := time.Now()
	termino := make(chan struct{})
	go func() { cmd.Wait(); close(termino) }()
	avisar(nav.Nombre)
	select {
	case <-termino:
		// Si terminó enseguida, le pasó la dirección a otro proceso con el mismo perfil.
		if time.Since(inicio) < 3*time.Second {
			return nil, ErrYaAbierto
		}
	case <-listo:
		cerrarBien(cmd, termino)
	case <-ctx.Done():
		cerrarBien(cmd, termino)
		return nil, ctx.Err()
	}

	// 3. Leer la sesión que dejó el sitio.
	cookies, err := conDevTools(ctx, nav, perfil, func(c *cliente) (map[string]string, error) {
		return c.cookiesDelSitio(ctx, hostDe(base))
	})
	if err != nil {
		return nil, fmt.Errorf("no pude leer la sesión de %s: %w", nav.Nombre, err)
	}
	if cookies["sid"] == "" {
		return nil, ErrNoEntro
	}
	return &Resultado{Cookies: cookies, Navegador: nav.Nombre}, nil
}

// banderas comunes a las dos formas de abrir el perfil. El almacén de claves tiene que ser el
// mismo en las dos (las cookies se guardan cifradas con él): uno fijo, sin llavero del sistema.
func banderas(perfil string) []string {
	b := []string{
		"--user-data-dir=" + perfil,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-default-apps",
	}
	switch runtime.GOOS {
	case "linux":
		b = append(b, "--password-store=basic")
	case "darwin":
		b = append(b, "--use-mock-keychain")
	}
	return b
}

// cerrarBien le pide al navegador que se cierre (guardando las cookies) y, si no, lo mata.
func cerrarBien(cmd *exec.Cmd, termino <-chan struct{}) {
	pedirCierre(cmd)
	select {
	case <-termino:
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		<-termino
	}
}

// conDevTools abre el perfil sin ventana, con DevTools solo en 127.0.0.1, corre f y lo cierra.
func conDevTools(ctx context.Context, nav Navegador, perfil string, f func(*cliente) (map[string]string, error)) (map[string]string, error) {
	puerto := filepath.Join(perfil, "DevToolsActivePort")
	os.Remove(puerto)
	cmd := exec.Command(nav.Ruta, append(banderas(perfil),
		"--headless=new",
		"--remote-debugging-port=0",
		"--remote-allow-origins=http://127.0.0.1",
		"about:blank",
	)...)
	prepararProceso(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	termino := make(chan struct{})
	go func() { cmd.Wait(); close(termino) }()
	defer func() {
		select {
		case <-termino:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-termino
		}
	}()

	var wsURL string
	limite := time.Now().Add(30 * time.Second)
	for wsURL == "" {
		if b, err := os.ReadFile(puerto); err == nil {
			lineas := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lineas) >= 2 {
				if p, err := strconv.Atoi(strings.TrimSpace(lineas[0])); err == nil && p > 0 {
					wsURL = fmt.Sprintf("ws://127.0.0.1:%d%s", p, strings.TrimSpace(lineas[1]))
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return nil, ctx.Err()
		case <-termino:
			return nil, ErrYaAbierto
		case <-time.After(150 * time.Millisecond):
		}
		if time.Now().After(limite) {
			cmd.Process.Kill()
			return nil, errors.New("no respondió")
		}
	}
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	conn.SetReadLimit(32 << 20)
	defer conn.CloseNow()
	c := &cliente{conn: conn}
	res, err := f(c)
	// Browser.close cierra ordenado: guarda en disco los cambios de cookies.
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.llamar(cctx, "Browser.close", nil)
	return res, err
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
	return c.llamarEn(ctx, "", metodo, params)
}

// llamarEn manda un comando a una sesión de una página (Target.attachToTarget con flatten).
func (c *cliente) llamarEn(ctx context.Context, sesion, metodo string, params any) (json.RawMessage, error) {
	c.id++
	pedido := map[string]any{"id": c.id, "method": metodo}
	if sesion != "" {
		pedido["sessionId"] = sesion
	}
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

// sesionDePagina se engancha a la pestaña (hace falta para el dominio Network).
func (c *cliente) sesionDePagina(ctx context.Context) (string, error) {
	var pagina string
	for i := 0; i < 50 && pagina == ""; i++ {
		res, err := c.llamar(ctx, "Target.getTargets", nil)
		if err != nil {
			return "", err
		}
		var r struct {
			TargetInfos []struct {
				TargetID string `json:"targetId"`
				Type     string `json:"type"`
			} `json:"targetInfos"`
		}
		json.Unmarshal(res, &r)
		for _, t := range r.TargetInfos {
			if t.Type == "page" {
				pagina = t.TargetID
				break
			}
		}
		if pagina == "" {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if pagina == "" {
		return "", errors.New("no apareció la pestaña")
	}
	res, err := c.llamar(ctx, "Target.attachToTarget", map[string]any{"targetId": pagina, "flatten": true})
	if err != nil {
		return "", err
	}
	var r struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(res, &r)
	return r.SessionID, nil
}

// borrarSesion borra la cookie sid del sitio del perfil.
func (c *cliente) borrarSesion(ctx context.Context, base string) error {
	s, err := c.sesionDePagina(ctx)
	if err != nil {
		return err
	}
	_, err = c.llamarEn(ctx, s, "Network.deleteCookies", map[string]any{"name": "sid", "url": base + "/"})
	return err
}

// cookiesDelSitio devuelve sid y visita del sitio, si están.
func (c *cliente) cookiesDelSitio(ctx context.Context, host string) (map[string]string, error) {
	cookies, err := c.cookies(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, ck := range cookies {
		if strings.TrimPrefix(ck.Domain, ".") == host && (ck.Name == "sid" || ck.Name == "visita") {
			out[ck.Name] = ck.Value
		}
	}
	return out, nil
}

// activos: logins con un navegador abierto (para esperarlos al cerrar la app).
var activos sync.WaitGroup

// Esperar da tiempo a que los logins en curso cierren su navegador (llamar después de cancelar
// su contexto, antes de terminar el programa).
func Esperar(max time.Duration) {
	listo := make(chan struct{})
	go func() { activos.Wait(); close(listo) }()
	select {
	case <-listo:
	case <-time.After(max):
	}
}

// perfilPara: el Chromium de Snap no puede escribir en carpetas ocultas del home (~/.cache);
// para ese se usa la carpeta que Snap le da (~/snap/chromium/common).
func perfilPara(nav Navegador, perfil string) string {
	real, err := filepath.EvalSymlinks(nav.Ruta)
	if err != nil {
		real = nav.Ruta
	}
	esSnap := strings.HasPrefix(real, "/snap/") || strings.Contains(real, "/snap/bin/")
	if !esSnap && runtime.GOOS == "linux" {
		// /usr/bin/chromium-browser de Ubuntu es un script que llama al snap.
		if b, err := os.ReadFile(real); err == nil && len(b) < 4096 && strings.Contains(string(b), "/snap/bin/") {
			esSnap = true
		}
	}
	if !esSnap {
		return perfil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return perfil
	}
	return filepath.Join(home, "snap", "chromium", "common", "txt421-navegador")
}
