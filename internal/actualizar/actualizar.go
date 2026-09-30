// Package actualizar baja la última versión publicada en GitHub y reemplaza el ejecutable.
package actualizar

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Repo de donde salen las versiones.
const Repo = "piojosso/txt421"

var cliente = &http.Client{Timeout: 2 * time.Minute}

// Ultima devuelve la última versión publicada ("0.2.0", sin la v). Usa la redirección de
// /releases/latest (no la API, que tiene un límite de pedidos sin cuenta).
func Ultima(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com/"+Repo+"/releases/latest", nil)
	c := *cliente
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := c.Do(req)
	if err != nil {
		return "", err
	}
	r.Body.Close()
	loc := r.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", errors.New("no hay versiones publicadas")
	}
	return strings.TrimPrefix(loc[i+len("/tag/"):], "v"), nil
}

// NombreArchivo del paquete para este sistema (el mismo que arman los releases).
func NombreArchivo() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("txt421_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

func bajar(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	r, err := cliente.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, r.Status)
	}
	return io.ReadAll(io.LimitReader(r.Body, 200<<20))
}

// Actualizar baja la última versión y reemplaza el ejecutable actual. Devuelve la versión.
func Actualizar(ctx context.Context, actual string) (string, error) {
	v, err := Ultima(ctx)
	if err != nil {
		return "", err
	}
	if v == actual {
		return v, nil
	}
	base := "https://github.com/" + Repo + "/releases/download/v" + v + "/"
	nombre := NombreArchivo()
	paquete, err := bajar(ctx, base+nombre)
	if err != nil {
		return "", err
	}
	sumas, err := bajar(ctx, base+"checksums.txt")
	if err != nil {
		return "", err
	}
	if err := verificar(paquete, sumas, nombre); err != nil {
		return "", err
	}
	bin, err := extraer(paquete, nombre)
	if err != nil {
		return "", err
	}
	if err := Reemplazar(bin); err != nil {
		return "", err
	}
	return v, nil
}

func verificar(datos, sumas []byte, nombre string) error {
	h := sha256.Sum256(datos)
	esperado := ""
	sc := bufio.NewScanner(bytes.NewReader(sumas))
	for sc.Scan() {
		campos := strings.Fields(sc.Text())
		if len(campos) == 2 && strings.TrimPrefix(campos[1], "*") == nombre {
			esperado = campos[0]
		}
	}
	if esperado == "" {
		return fmt.Errorf("%s no figura en checksums.txt", nombre)
	}
	if hex.EncodeToString(h[:]) != esperado {
		return errors.New("la descarga no coincide con su checksum: no se instaló")
	}
	return nil
}

func extraer(paquete []byte, nombre string) ([]byte, error) {
	exe := "txt421"
	if runtime.GOOS == "windows" {
		exe = "txt421.exe"
	}
	if strings.HasSuffix(nombre, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(paquete), int64(len(paquete)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == exe {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, errors.New("el paquete no trae " + exe)
	}
	gz, err := gzip.NewReader(bytes.NewReader(paquete))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("el paquete no trae " + exe)
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == exe && h.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

// Reemplazar pone bin en lugar del ejecutable que está corriendo. En Windows no se puede pisar
// un .exe en uso, pero sí renombrarlo: el viejo queda como .old y se borra la próxima vez.
func Reemplazar(bin []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	nuevo := exe + ".new"
	if err := os.WriteFile(nuevo, bin, 0o755); err != nil {
		return fmt.Errorf("no puedo escribir en %s: %w", filepath.Dir(exe), err)
	}
	viejo := exe + ".old"
	os.Remove(viejo)
	if err := os.Rename(exe, viejo); err != nil {
		os.Remove(nuevo)
		return err
	}
	if err := os.Rename(nuevo, exe); err != nil {
		os.Rename(viejo, exe)
		return err
	}
	if runtime.GOOS != "windows" {
		os.Remove(viejo)
	}
	return nil
}

// Limpiar borra el .old que deja una actualización en Windows.
func Limpiar() {
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}
