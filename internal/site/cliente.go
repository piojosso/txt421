package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BaseURL es la dirección del sitio (TXT421_BASE la cambia, para pruebas).
var BaseURL = func() string {
	if b := os.Getenv("TXT421_BASE"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://txt.421.news"
}()

// Errores que la interfaz distingue.
var (
	ErrNoEncontrado = errors.New("no existe o ya no está disponible")
	ErrSesion       = errors.New("hace falta entrar")
)

// ErrSitio es un error que el sitio explicó con palabras (formulario rechazado, límite, etc.).
type ErrSitio struct {
	Status  int
	Mensaje string
}

func (e *ErrSitio) Error() string { return e.Mensaje }

// ErrRed es un problema de conexión.
type ErrRed struct{ Causa error }

func (e *ErrRed) Error() string {
	return "sin conexión con " + hostDe(BaseURL) + ": " + e.Causa.Error()
}
func (e *ErrRed) Unwrap() error { return e.Causa }

func hostDe(u string) string {
	if p, err := url.Parse(u); err == nil {
		return p.Host
	}
	return u
}

// Cliente habla con el sitio. Guarda las cookies del sitio (la sesión `sid` y `visita`, con la
// que el sitio marca lo nuevo desde tu visita anterior) en memoria; quien lo usa decide si las
// persiste (Cookies/SetCookies).
type Cliente struct {
	Base      string
	UserAgent string
	http      *http.Client

	mu      sync.Mutex
	cookies map[string]string
	csrf    string
}

// Nuevo crea un cliente para BaseURL.
func Nuevo(userAgent string) *Cliente {
	return &Cliente{
		Base:      BaseURL,
		UserAgent: userAgent,
		cookies:   map[string]string{},
		http: &http.Client{
			// Las redirecciones se manejan a mano: dicen si una publicación salió y adónde.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
				TLSHandshakeTimeout: 15 * time.Second,
				IdleConnTimeout:     90 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
	}
}

// Cookies devuelve las cookies del sitio (para guardarlas).
func (c *Cliente) Cookies() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.cookies))
	for k, v := range c.cookies {
		out[k] = v
	}
	return out
}

// SetCookies reemplaza las cookies (al cargar lo guardado o al entrar).
func (c *Cliente) SetCookies(m map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cookies = map[string]string{}
	for k, v := range m {
		if v != "" {
			c.cookies[k] = v
		}
	}
	c.csrf = ""
}

// Sesion devuelve la cookie de sesión ("" si no hay).
func (c *Cliente) Sesion() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cookies["sid"]
}

func (c *Cliente) cabeceraCookies() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var partes []string
	for k, v := range c.cookies {
		partes = append(partes, k+"="+v)
	}
	return strings.Join(partes, "; ")
}

func (c *Cliente) guardarCookies(r *http.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ck := range r.Cookies() {
		if ck.MaxAge < 0 || ck.Value == "" || (!ck.Expires.IsZero() && ck.Expires.Before(time.Now())) {
			delete(c.cookies, ck.Name)
			continue
		}
		c.cookies[ck.Name] = ck.Value
	}
}

type respuesta struct {
	status  int
	cuerpo  string
	destino string // Location, si es una redirección
}

func (c *Cliente) hacer(ctx context.Context, metodo, ruta string, form url.Values) (*respuesta, error) {
	u := ruta
	if !strings.HasPrefix(u, "http") {
		u = c.Base + ruta
	}
	var cuerpo io.Reader
	if form != nil {
		cuerpo = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, metodo, u, cuerpo)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/json;q=0.9")
	req.Header.Set("Accept-Language", "es-AR,es;q=0.9")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	// La sesión va solo al sitio (y no por http si el sitio es https), aunque una redirección
	// lleve a otro lado.
	if base, err := url.Parse(c.Base); err == nil && req.URL.Host == base.Host && req.URL.Scheme == base.Scheme {
		if ck := c.cabeceraCookies(); ck != "" {
			req.Header.Set("Cookie", ck)
		}
	}
	r, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &ErrRed{Causa: err}
	}
	defer r.Body.Close()
	c.guardarCookies(r)
	datos, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return nil, &ErrRed{Causa: err}
	}
	res := &respuesta{status: r.StatusCode, cuerpo: string(datos)}
	if r.StatusCode >= 300 && r.StatusCode < 400 {
		if loc, err := r.Location(); err == nil {
			res.destino = loc.String()
		}
	}
	return res, nil
}

// get pide una página y sigue las redirecciones. /entrar como destino = hace falta sesión.
func (c *Cliente) get(ctx context.Context, ruta string) (string, error) {
	for i := 0; i < 5; i++ {
		r, err := c.hacer(ctx, http.MethodGet, ruta, nil)
		if err != nil {
			return "", err
		}
		switch {
		case r.destino != "":
			if strings.HasSuffix(strings.SplitN(r.destino, "?", 2)[0], "/entrar") {
				return "", ErrSesion
			}
			ruta = r.destino
			continue
		case r.status == http.StatusNotFound:
			return "", ErrNoEncontrado
		case r.status == http.StatusTooManyRequests:
			return "", &ErrSitio{Status: r.status, Mensaje: "El sitio pide ir más despacio. Probá en un rato."}
		case r.status >= 400:
			msg := ParseError(r.cuerpo)
			if msg == "" {
				msg = fmt.Sprintf("El sitio respondió %d.", r.status)
			}
			return "", &ErrSitio{Status: r.status, Mensaje: msg}
		}
		c.recordarCSRF(r.cuerpo)
		return r.cuerpo, nil
	}
	return "", &ErrSitio{Mensaje: "Demasiadas redirecciones."}
}

var reCSRF = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

func (c *Cliente) recordarCSRF(doc string) {
	if m := reCSRF.FindStringSubmatch(doc); m != nil {
		c.mu.Lock()
		c.csrf = m[1]
		c.mu.Unlock()
	}
}

func (c *Cliente) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	t := c.csrf
	c.mu.Unlock()
	if t != "" {
		return t, nil
	}
	cab, err := c.Estado(ctx)
	if err != nil {
		return "", err
	}
	if !cab.Conectado || cab.CSRF == "" {
		return "", ErrSesion
	}
	return cab.CSRF, nil
}

func conPagina(ruta string, pagina int, extra url.Values) string {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	if pagina > 1 {
		q.Set("pagina", strconv.Itoa(pagina))
	}
	if len(q) == 0 {
		return ruta
	}
	return ruta + "?" + q.Encode()
}

// ─── Lectura ────────────────────────────────────────────────────────────────────────────────

// Estado pide la portada solo para saber si la sesión sirve y cuántas respuestas nuevas hay.
func (c *Cliente) Estado(ctx context.Context) (Cabecera, error) {
	doc, err := c.get(ctx, "/normas")
	if err != nil {
		return Cabecera{}, err
	}
	return leerCabecera(parsear(doc)), nil
}

// Portada: vista "catalogo" (60 por página) o "lista" (10 por página, con las últimas respuestas).
func (c *Cliente) Portada(ctx context.Context, pagina int, vista string) (*Listado, error) {
	doc, err := c.get(ctx, conPagina("/", pagina, url.Values{"vista": {vista}}))
	if err != nil {
		return nil, err
	}
	return ParseListado(doc), nil
}

// Seccion: una sección o su archivo.
func (c *Cliente) Seccion(ctx context.Context, slug string, archivo bool, pagina int, vista string) (*Listado, error) {
	ruta := "/b/" + url.PathEscape(slug)
	if archivo {
		ruta += "/archivo"
	}
	doc, err := c.get(ctx, conPagina(ruta, pagina, url.Values{"vista": {vista}}))
	if err != nil {
		return nil, err
	}
	return ParseListado(doc), nil
}

// Hilo: una publicación entera.
func (c *Cliente) Hilo(ctx context.Context, no int) (*Hilo, error) {
	doc, err := c.get(ctx, "/h/"+strconv.Itoa(no))
	if err != nil {
		return nil, err
	}
	return ParseHilo(doc, no), nil
}

// Nuevos: los mensajes de una publicación posteriores a `desde` (lo que usa la web para
// actualizarse sola cada 20 segundos).
func (c *Cliente) Nuevos(ctx context.Context, no, desde int) (int, []Post, error) {
	r, err := c.hacer(ctx, http.MethodGet, fmt.Sprintf("/h/%d/nuevos?desde=%d", no, desde), nil)
	if err != nil {
		return desde, nil, err
	}
	if r.status == http.StatusNotFound {
		return desde, nil, ErrNoEncontrado
	}
	if r.status != http.StatusOK {
		return desde, nil, &ErrSitio{Status: r.status, Mensaje: fmt.Sprintf("El sitio respondió %d.", r.status)}
	}
	var datos struct {
		Ultimo int    `json:"ultimo"`
		HTML   string `json:"html"`
	}
	if err := json.Unmarshal([]byte(r.cuerpo), &datos); err != nil {
		return desde, nil, &ErrSitio{Mensaje: "Respuesta inesperada del sitio."}
	}
	return max(desde, datos.Ultimo), ParseNuevos(datos.HTML), nil
}

// Buscar en todo el foro (incluye el archivo).
func (c *Cliente) Buscar(ctx context.Context, consulta string, pagina int) (*Busqueda, error) {
	doc, err := c.get(ctx, conPagina("/buscar", pagina, url.Values{"q": {consulta}}))
	if err != nil {
		return nil, err
	}
	return ParseBusqueda(doc, consulta), nil
}

// Ubicar: de un No. de mensaje a su publicación.
func (c *Cliente) Ubicar(ctx context.Context, post int) (hilo int, err error) {
	r, err := c.hacer(ctx, http.MethodGet, "/p/"+strconv.Itoa(post), nil)
	if err != nil {
		return 0, err
	}
	if m := regexp.MustCompile(`/h/(\d+)`).FindStringSubmatch(r.destino); m != nil {
		return strconv.Atoi(m[1])
	}
	return 0, ErrNoEncontrado
}

// Bandeja: /respuestas. Pedirla marca los avisos como leídos, como en la web.
func (c *Cliente) Bandeja(ctx context.Context) (*Bandeja, error) {
	doc, err := c.get(ctx, "/respuestas")
	if err != nil {
		return nil, err
	}
	return ParseBandeja(doc), nil
}

// Guardados: /guardados.
func (c *Cliente) Guardados(ctx context.Context) (*Guardados, error) {
	doc, err := c.get(ctx, "/guardados")
	if err != nil {
		return nil, err
	}
	return ParseGuardados(doc), nil
}

// Documento: normas, formato, términos, privacidad.
func (c *Cliente) Documento(ctx context.Context, ruta string) (*Documento, error) {
	doc, err := c.get(ctx, ruta)
	if err != nil {
		return nil, err
	}
	return ParseDocumento(doc), nil
}

// ─── Escritura ──────────────────────────────────────────────────────────────────────────────

// Publicado dice adónde quedó un mensaje.
type Publicado struct {
	Hilo       int
	Post       int  // 0 si es una publicación nueva (su mensaje es el inicial)
	EnRevision bool // el filtro tuvo dudas: lo mira una persona; mientras tanto solo lo ves vos
}

var reDestino = regexp.MustCompile(`/h/(\d+)(\?[^#]*)?(?:#p(\d+))?`)

// enviar manda un formulario. Éxito = redirección (303); si no, el sitio redibuja la página con
// el error, que se devuelve como ErrSitio.
func (c *Cliente) enviar(ctx context.Context, ruta string, form url.Values) (*respuesta, error) {
	t, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	form.Set("_csrf", t)
	r, err := c.hacer(ctx, http.MethodPost, ruta, form)
	if err != nil {
		return nil, err
	}
	if r.destino != "" {
		if strings.HasSuffix(strings.SplitN(r.destino, "?", 2)[0], "/entrar") {
			return nil, ErrSesion
		}
		return r, nil
	}
	if r.status == http.StatusNotFound {
		return nil, ErrNoEncontrado
	}
	if r.status == http.StatusForbidden {
		// "Formulario vencido": el token cambió (se volvió a entrar). Se olvida para pedirlo de nuevo.
		c.mu.Lock()
		c.csrf = ""
		c.mu.Unlock()
	}
	msg := ParseError(r.cuerpo)
	if msg == "" {
		msg = fmt.Sprintf("El sitio respondió %d.", r.status)
	}
	return nil, &ErrSitio{Status: r.status, Mensaje: msg}
}

func publicado(destino string) (*Publicado, error) {
	m := reDestino.FindStringSubmatch(destino)
	if m == nil {
		// "/" = la cuenta ya no puede publicar (borrada o suspendida mientras se moderaba).
		return nil, &ErrSitio{Mensaje: "Tu cuenta no puede publicar."}
	}
	p := &Publicado{EnRevision: strings.Contains(m[2], "aviso=cola")}
	p.Hilo, _ = strconv.Atoi(m[1])
	p.Post, _ = strconv.Atoi(m[3])
	return p, nil
}

// Responder en una publicación. El filtro del sitio puede tardar unos segundos.
func (c *Cliente) Responder(ctx context.Context, hilo int, cuerpo string, sage bool) (*Publicado, error) {
	form := url.Values{"cuerpo": {cuerpo}}
	if sage {
		form.Set("sage", "1")
	}
	r, err := c.enviar(ctx, fmt.Sprintf("/h/%d/responder", hilo), form)
	if err != nil {
		return nil, err
	}
	return publicado(r.destino)
}

// Publicar una publicación nueva en una sección.
func (c *Cliente) Publicar(ctx context.Context, seccion, asunto, cuerpo string) (*Publicado, error) {
	r, err := c.enviar(ctx, "/b/"+url.PathEscape(seccion)+"/hilo", url.Values{"asunto": {asunto}, "cuerpo": {cuerpo}})
	if err != nil {
		return nil, err
	}
	return publicado(r.destino)
}

// Guardar o sacar de guardados.
func (c *Cliente) Guardar(ctx context.Context, hilo int, quitar bool) error {
	form := url.Values{}
	if quitar {
		form.Set("quitar", "1")
	}
	_, err := c.enviar(ctx, fmt.Sprintf("/h/%d/guardar", hilo), form)
	return err
}

// Motivos para reportar un mensaje (las normas). Vacío si no se puede reportar (es tuyo).
func (c *Cliente) Motivos(ctx context.Context, post int) ([]Motivo, error) {
	doc, err := c.get(ctx, fmt.Sprintf("/p/%d/reportar", post))
	if err != nil {
		return nil, err
	}
	return ParseMotivos(doc), nil
}

// Reportar un mensaje.
func (c *Cliente) Reportar(ctx context.Context, post int, motivo string) error {
	r, err := c.enviar(ctx, fmt.Sprintf("/p/%d/reportar", post), url.Values{"motivo": {motivo}})
	if err != nil {
		return err
	}
	// El sitio vuelve a la publicación con ?aviso=reportado; sin eso, no se guardó (mensaje
	// propio o cuenta suspendida).
	if !strings.Contains(r.destino, "aviso=reportado") {
		return &ErrSitio{Mensaje: "No se pudo reportar este mensaje."}
	}
	return nil
}

// PuedeBorrar dice si un mensaje propio se puede borrar ya, o por qué no.
func (c *Cliente) PuedeBorrar(ctx context.Context, post int) (motivo string, err error) {
	doc, err := c.get(ctx, fmt.Sprintf("/p/%d/borrar", post))
	if err != nil {
		return "", err
	}
	return ParseBorrar(doc), nil
}

// Borrar un mensaje propio. No se puede deshacer.
func (c *Cliente) Borrar(ctx context.Context, post int) error {
	r, err := c.enviar(ctx, fmt.Sprintf("/p/%d/borrar", post), url.Values{})
	if err != nil {
		return err
	}
	if strings.Contains(r.destino, "/borrar") {
		return &ErrSitio{Mensaje: "Este mensaje está en revisión. Se va a poder borrar cuando lo resuelva un moderador."}
	}
	return nil
}

// Salir cierra la sesión en el sitio.
func (c *Cliente) Salir(ctx context.Context) error {
	_, err := c.enviar(ctx, "/salir", url.Values{})
	c.mu.Lock()
	delete(c.cookies, "sid")
	c.csrf = ""
	c.mu.Unlock()
	return err
}
