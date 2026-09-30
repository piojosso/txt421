// Package config guarda lo que txt421 recuerda entre usos: preferencias, la sesión del sitio y
// los borradores. Todo en el directorio de configuración del sistema:
//
//	Linux    ~/.config/txt421
//	macOS    ~/Library/Application Support/txt421
//	Windows  %AppData%\txt421
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Dir es el directorio de configuración (TXT421_CONFIG lo cambia, para pruebas).
func Dir() string {
	if d := os.Getenv("TXT421_CONFIG"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.TempDir(), "txt421-"+os.Getenv("USER"))
	}
	return filepath.Join(base, "txt421")
}

// DirPerfil es donde vive el perfil del navegador que se usa para entrar (así Google recuerda
// la cuenta la próxima vez). Va en la caché: se puede borrar sin perder nada importante.
func DirPerfil() string {
	if d := os.Getenv("TXT421_CONFIG"); d != "" {
		return filepath.Join(d, "navegador")
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(Dir(), "navegador")
	}
	return filepath.Join(base, "txt421", "navegador")
}

// Preferencias del usuario.
type Preferencias struct {
	Tema   string `json:"tema"`   // oscuro, claro, descanso, monocromo
	Vista  string `json:"vista"`  // catalogo, lista
	Idioma string `json:"idioma"` // "", es, en ("" = el del sistema)
	// Ultima versión que se avisó que había para actualizar (para no insistir).
	AvisoVersion  string    `json:"aviso_version,omitempty"`
	UltimoChequeo time.Time `json:"ultimo_chequeo,omitempty"`
}

// Borrador de un formulario (como el sessionStorage de la web: dura un día).
type Borrador struct {
	Seccion string    `json:"seccion,omitempty"`
	Asunto  string    `json:"asunto,omitempty"`
	Cuerpo  string    `json:"cuerpo"`
	Sage    bool      `json:"sage,omitempty"`
	T       time.Time `json:"t"`
}

// ValidezBorrador: pasado esto, un borrador no se vuelve a ofrecer (igual que la web).
const ValidezBorrador = 24 * time.Hour

var mu sync.Mutex

func leer(nombre string, v any) error {
	b, err := os.ReadFile(filepath.Join(Dir(), nombre))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// escribir guarda de forma atómica (archivo temporal + rename), con permisos solo para el dueño.
func escribir(nombre string, v any) error {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(Dir(), nombre)
	tmp, err := os.CreateTemp(Dir(), nombre+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	os.Chmod(tmp.Name(), 0o600)
	if err := os.Rename(tmp.Name(), final); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// CargarPreferencias devuelve lo guardado, con los valores por defecto donde falte.
func CargarPreferencias() Preferencias {
	var p Preferencias
	_ = leer("preferencias.json", &p)
	switch p.Tema {
	case "oscuro", "claro", "descanso", "monocromo":
	default:
		p.Tema = "oscuro"
	}
	if p.Vista != "lista" {
		p.Vista = "catalogo"
	}
	return p
}

// GuardarPreferencias las escribe.
func GuardarPreferencias(p Preferencias) error { return escribir("preferencias.json", p) }

// CargarCookies devuelve las cookies del sitio guardadas (sesión y visita).
func CargarCookies() map[string]string {
	m := map[string]string{}
	_ = leer("sesion.json", &m)
	return m
}

// GuardarCookies las escribe (0600: la sesión da acceso a tu cuenta del foro).
func GuardarCookies(m map[string]string) error { return escribir("sesion.json", m) }

// CargarBorradores devuelve los borradores vigentes, por formulario ("respuesta:123", "nuevo").
func CargarBorradores() map[string]Borrador {
	m := map[string]Borrador{}
	_ = leer("borradores.json", &m)
	for k, b := range m {
		if time.Since(b.T) > ValidezBorrador || (b.Cuerpo == "" && b.Asunto == "") {
			delete(m, k)
		}
	}
	return m
}

// GuardarBorradores los escribe.
func GuardarBorradores(m map[string]Borrador) error { return escribir("borradores.json", m) }
