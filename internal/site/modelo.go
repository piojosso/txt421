// Package site habla con txt.421.news: lee sus páginas HTML y publica con los mismos formularios
// que la web. No hay API: el HTML del sitio (github.com/421news/txt, src/views.js) es el contrato.
package site

import "time"

// Estilo de un tramo de texto dentro de un mensaje.
type Estilo int

const (
	Normal  Estilo = iota
	Cita           // línea que empieza con > (class="verde")
	Ref            // >>123 (a.cita)
	Spoiler        // [spoiler]…[/spoiler]
	Marca          // coincidencia de búsqueda (<mark>)
	Salto          // fin de línea
)

// Tramo es un pedazo de texto con un estilo. Un mensaje es una lista de tramos.
type Tramo struct {
	Estilo Estilo
	Texto  string
	// Para Ref: el número de mensaje citado.
	Num int
}

// Seccion es un tablón del foro.
type Seccion struct {
	Slug, Nombre, Descripcion string
}

// Secciones en el orden del sitio (src/config.js).
var Secciones = []Seccion{
	{"tecnologia", "Tecnología", "Software, hardware, internet y ciencia."},
	{"cultura", "Cultura", "Libros, cine, arte, historia e ideas."},
	{"musica", "Música", "Discos, bandas, recitales, escenas y lo que estás escuchando."},
	{"juegos", "Juegos", "Videojuegos, juegos de mesa, rol y cartas."},
	{"vida-real", "Vida real", "Todo lo que NO sucede a través de una pantalla."},
}

// SeccionPorSlug devuelve la sección o nil.
func SeccionPorSlug(slug string) *Seccion {
	for i := range Secciones {
		if Secciones[i].Slug == slug {
			return &Secciones[i]
		}
	}
	return nil
}

// SeccionPorNombre busca por el nombre que muestra el sitio ("Vida real").
func SeccionPorNombre(nombre string) *Seccion {
	for i := range Secciones {
		if Secciones[i].Nombre == nombre {
			return &Secciones[i]
		}
	}
	return nil
}

// Límites del sitio (src/config.js).
const (
	MaxAsunto     = 120
	MaxCuerpo     = 8000
	SegEntrePosts = 30
	SegEntreHilos = 600
)

// Cabecera: lo que la barra de arriba dice de la sesión.
type Cabecera struct {
	Conectado  bool
	Novedades  int    // el número del sobre (Respuestas nuevas)
	CSRF       string // token de los formularios (fijo por sesión)
	Pendientes int    // moderación (solo mods)
}

// Ficha es una publicación en el catálogo.
type Ficha struct {
	Hilo     int
	Asunto   string
	Extracto string
	R        int    // respuestas ("R: N")
	Seccion  string // nombre de la sección (en la portada) o "" en un tablón
	OpNo     int    // No. del mensaje inicial (en un tablón)
	Fijada   bool
	Novedad  string // "nuevo", "2 nuevas" (desde tu visita anterior)
	Fecha    string // tal como la muestra el sitio (último movimiento)
	Cerrada  bool
	Resumen  *Resumen // solo en la vista lista
}

// Resumen es una publicación en la vista lista: el mensaje inicial y las últimas respuestas.
type Resumen struct {
	Op       Post
	Omitidas int
	Ultimas  []Post
	Etiqueta string // la sección, en la portada
	Pie      string // "Responder · 12 respuestas · cerrada"
}

// Listado es una página de la portada, de una sección o de su archivo.
type Listado struct {
	Cabecera
	Titulo      string // "Juegos", "Juegos · archivo"; "" en la portada
	Descripcion string
	Fichas      []Ficha
	Pagina      int
	Paginas     int
	Vista       string // "catalogo" o "lista"
	Aviso       string
}

// Post es un mensaje de una publicación.
type Post struct {
	No             int
	Autor          string // ID dentro de la publicación
	Op             bool   // lo escribió quien abrió la publicación
	Inicial        bool   // es el mensaje inicial
	Vos            bool   // (vos): lo escribiste vos
	Nuevo          bool   // desde tu visita anterior
	Sage           bool
	Cuando         time.Time
	Fecha          string // como la muestra el sitio
	Cuerpo         []Tramo
	Respuestas     []int
	Retirado       string // "Eliminado por su autor." u otro motivo: no hay texto
	EnRevision     bool   // en revisión: por ahora solo lo ves vos
	PuedeBorrar    bool
	PuedeReportar  bool
	PuedeResponder bool
}

// Hilo es una publicación entera.
type Hilo struct {
	Cabecera
	No           int
	Asunto       string
	Seccion      string // slug
	Posts        []Post
	Ultimo       int
	Abierto      bool   // acepta respuestas
	Estado       string // "archivada", "cerrada" o ""
	EstadoTxt    string // el aviso del sitio
	Fijada       bool
	Guardado     bool
	PuedeGuardar bool
	Aviso        string // aviso de la página (?aviso=)
}

// Respuestas cuenta las respuestas visibles como el "R:" del sitio: sin el inicial ni los retirados.
func (h *Hilo) Respuestas() int {
	n := 0
	for _, p := range h.Posts {
		if !p.Inicial && p.Retirado == "" {
			n++
		}
	}
	return n
}

// Resultado de una búsqueda.
type Resultado struct {
	Hilo      int
	Post      int
	Asunto    string
	Info      string // "Juegos · No.1488 · 24/9/26, 6:52 p. m."
	Fragmento []Tramo
}

// Busqueda es una página de resultados.
type Busqueda struct {
	Cabecera
	Consulta   string
	Total      int
	Resultados []Resultado
	Pagina     int
	Paginas    int
}

// Aviso de /respuestas.
type Aviso struct {
	Hilo     int
	Post     int
	Asunto   string
	Info     string // "· alguien te respondió · 30/9/26, 1:02 a. m. · nueva"
	Extracto string
	Nueva    bool
}

// Enlace a una publicación con su línea de datos (Donde participaste, Guardados).
type Enlace struct {
	Hilo   int
	Asunto string
	Info   string
}

// Bandeja es /respuestas.
type Bandeja struct {
	Cabecera
	Avisos []Aviso
	Mias   []Enlace
}

// Guardados es /guardados.
type Guardados struct {
	Cabecera
	Lista []Enlace
}

// Documento es una página de texto del sitio (normas, formato) en bloques simples.
type Documento struct {
	Titulo  string
	Bloques []Bloque
}

// Bloque de un documento.
type Bloque struct {
	Tipo  string // "h2", "p", "li", "pre", "ayuda"
	Texto []Tramo
	Num   int // número de ítem en listas ordenadas
}

// Motivo para reportar (las normas).
type Motivo struct {
	ID, Titulo string
}
