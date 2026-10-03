# txt421

Un cliente de [txt.421.news](https://txt.421.news) que corre en la terminal.

txt es el foro de texto de [421](https://www.421.news). Solo texto, pseudoanónimo, sin likes, sin algoritmo. O sea, básicamente un foro que ya se veía como una terminal. Faltaba que ande en CLI, así que hice eso.

Se ve igual que el sitio (los mismos colores, las mismas fichas, los mismos temas), se maneja con las flechitas y NO tiene scroll: todo va en páginas del tamaño de la ventana. Podés leer, publicar, responder, guardar, reportar y borrar lo tuyo. Todo lo que ya hacés en la web, pero con más cara de hacker.

![Una publicación en txt421](docs/publicacion-oscuro.png)

## Instalación

Un solo comando. No hace falta instalar nada antes (ni Python, ni Node, ni nada).

**Mac o Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/piojosso/txt421/main/install.sh | sh
```

**Windows** (en PowerShell)

```powershell
irm https://raw.githubusercontent.com/piojosso/txt421/main/install.ps1 | iex
```

Después abrís una terminal nueva y escribís `txt421`. Listo, eso es todo.

Para actualizar: `txt421 actualizar`. Igual cuando hay una versión nueva te avisa solo, y la podés instalar desde el menú.

<details>
<summary>¿Qué hace exactamente el instalador?</summary>

Baja el ejecutable de la última versión desde [Releases](https://github.com/piojosso/txt421/releases), chequea que el checksum coincida y lo deja en `~/.local/bin` (Mac y Linux) o en `%LOCALAPPDATA%\Programs\txt421` (Windows). Si esa carpeta no está en tu PATH, la agrega. Nada más. Si preferís, podés bajar los archivos a mano desde Releases.
</details>

## Cómo se usa (subcomandos de la consola)

```
txt421                 Abre la app en la portada
txt421 1542            abre la publicación 1542 (también podés pegar el link)
txt421 b juegos        una sección
txt421 buscar algo     busca en todo el foro
txt421 respuestas      tus respuestas
txt421 entrar          entrar con tu cuenta
```

Adentro:

| Tecla | Qué hace |
|---|---|
| Flechitas | moverte entre las fichas y los mensajes |
| Enter | abrir. En un mensaje, responderle (te escribe el `>>N`) |
| ← → · PgUp PgDn | cambiar de página |
| Tab · 0–5 | cambiar de sección (0 es la portada) |
| Esc | volver |
| `n` | publicar |
| `r` | responder |
| `i` · click en un `>>N` | te muestra el mensaje citado (o las respuestas) flotando, como en la web. Enter te lleva hasta ahí y Esc te trae de vuelta |
| `g` | guardar la publicación |
| `x` | reportar (o borrar, si es tuyo) |
| `s` | destapar el spoiler |
| `v` | vista catálogo o lista |
| `/` | buscar |
| `m` | el menú (Respuestas, Guardados, Preferencias, Normas…) |
| `t` | cambiar el tema (oscuro, claro, descanso, monocromo) |
| `?` | todas las teclas, por si te olvidás |

Cuando estás escribiendo: Tab pasa de un campo a otro, **Ctrl+Enter** (o **Ctrl+S**) publica, Ctrl+P te muestra la vista previa y Esc vuelve (y te guarda el borrador). El mouse también funciona, para los que no se bancan vivir sin mouse (o sea para los que no son gordos programadores como yo).

Las publicaciones que tenés abiertas se actualizan solas cada 20 segundos, igual que en la web.

## Entrar

Como el único login de la web es con Google, y Google solo te deja logear desde un navegador de verdad, y una terminal no es un navegador de verdad (todavía).

Entonces `txt421 entrar` te abre una ventana de Chrome, Edge, Brave o Chromium, con un perfil aparte que es solo de txt421. Entrás como siempre, y cuando ya ves el foro cerrás esa ventana (o volvés a la terminal y apretás Enter). Ahí txt421 abre ese mismo perfil sin ventana y agarra la sesión que dejó el sitio. La sesión dura 30 días, y la próxima vez Google ya se acuerda de tu cuenta, así que es más rápido.

¿Por qué no lee la sesión directo con la ventana abierta? Porque lo intenté, y Google se dio cuenta y me dijo que mi navegador "puede no ser seguro". Muy amable Google.

Si no tenés ninguno de esos navegadores, o si Google igual se pone densa, está la opción de pegar la cookie a mano: en un navegador donde ya entraste a txt.421.news abrís las herramientas de desarrollo (F12) → Almacenamiento → Cookies → txt.421.news, y copiás el valor de `sid`.

La sesión se guarda solamente en tu compu (en `~/.config/txt421`, `~/Library/Application Support/txt421` o `%AppData%\txt421`, según tu sistema) y solo tu usuario la puede leer. txt421 se comunica solamente con txt.421.news y con github para ver si hay actualizaciones. No hay nada más.

## Cómo funciona

No hay API (todavía, o nunca, no sé). txt421 lee el mismo HTML que ve tu navegador y publica usando los mismos formularios que la web. Eso significa que todo pasa por las mismas normas, el mismo filtro de moderación y los mismos límites: 30 segundos entre mensajes y 10 minutos entre publicaciones.

El código del sitio es abierto, y la verdad que eso ayudó muchísimo: [github.com/421news/txt](https://github.com/421news/txt).

Está hecho en Go con [Bubble Tea](https://github.com/charmbracelet/bubbletea). Si lo querés compilar vos:

```sh
go build ./cmd/txt421
go test ./...
```

`TXT421_BASE` lo apunta a otro servidor (por ejemplo, el sitio corriendo en tu compu) y `TXT421_LANG=en` pone la interfaz en inglés.

---

## English

made a client for [txt.421.news](https://txt.421.news) that runs in the terminal. it looks like the site, you move around with the arrow keys, and there's no scrolling, everything goes in pages. you can read, post, reply, save, report and delete your own stuff.

installs on windows, mac or linux with a single command (the ones above). the interface follows your system language (spanish or english) and you can change it in Preferences. press `?` inside for all the keys.

![Replying](docs/responder.png)

---

Licencia MIT. txt421 no es un producto oficial de 421, lo hice yo porque quería leer el foro desde la terminal. / MIT license. not an official 421 thing.
