// Package i18n: la interfaz en castellano (como el sitio) o en inglés. Lo que escribe la gente y
// los mensajes que manda el sitio quedan como vienen.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

var idioma = "es"

// Usar fija el idioma: "es", "en" o "" (el del sistema).
func Usar(i string) {
	switch i {
	case "es", "en":
		idioma = i
	default:
		idioma = Sistema()
	}
}

// Actual devuelve el idioma en uso.
func Actual() string { return idioma }

// Sistema adivina el idioma del sistema; castellano si no se sabe.
func Sistema() string {
	for _, v := range []string{"TXT421_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if l := os.Getenv(v); l != "" && l != "C" && l != "POSIX" {
			return deCodigo(l)
		}
	}
	if l := idiomaSO(); l != "" {
		return deCodigo(l)
	}
	return "es"
}

func deCodigo(l string) string {
	if strings.HasPrefix(strings.ToLower(l), "en") {
		return "en"
	}
	return "es"
}

// T traduce una clave; con argumentos, formatea como fmt.Sprintf.
func T(clave string, args ...any) string {
	t, ok := textos[clave]
	s := clave
	if ok {
		s = t[0]
		if idioma == "en" && t[1] != "" {
			s = t[1]
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// textos: clave → {castellano, inglés}. El castellano es el del sitio siempre que existe.
var textos = map[string][2]string{
	// Cabecera y navegación
	"leer421":      {"Leé 421 ↗", "Read 421 ↗"},
	"buscar":       {"Buscar", "Search"},
	"entrar":       {"Entrar", "Log in"},
	"menu":         {"Menú", "Menu"},
	"portada":      {"Portada", "Front page"},
	"respuestas":   {"Respuestas", "Replies"},
	"guardados":    {"Guardados", "Saved"},
	"preferencias": {"Preferencias", "Preferences"},
	"normas":       {"Normas", "Rules"},
	"formato":      {"Formato", "Formatting"},
	"salir":        {"Salir", "Log out"},
	"cerrar_app":   {"Cerrar txt421", "Quit txt421"},
	"actualizar":   {"Actualizar txt421", "Update txt421"},
	"abrir_web":    {"Abrir en la web", "Open in browser"},

	// Listados
	"publicar":          {"Publicar", "Post"},
	"publicar_en":       {"Publicar en %s", "Post in %s"},
	"moderado":          {"Pseudoanónimo y moderado: cada mensaje se revisa antes de publicarse.", "Pseudonymous and moderated: every message is reviewed before it's published."},
	"vista":             {"Vista:", "View:"},
	"catalogo":          {"catálogo", "catalog"},
	"lista":             {"lista", "list"},
	"archivo":           {"Archivo", "Archive"},
	"volver_activas":    {"Volver a las publicaciones activas", "Back to active threads"},
	"sin_publicaciones": {"No hay publicaciones todavía.", "No threads yet."},
	"fijada":            {"Fijada", "Pinned"},
	"cerrada":           {"cerrada", "closed"},
	"omitidas_1":        {"1 respuesta omitida.", "1 reply omitted."},
	"omitidas_n":        {"%d respuestas omitidas.", "%d replies omitted."},
	"ver_completa":      {"Ver la publicación completa", "See the whole thread"},
	"pagina_de":         {"Página %d de %d", "Page %d of %d"},
	"anterior":          {"← Anterior", "← Previous"},
	"siguiente":         {"Siguiente →", "Next →"},
	"entra_para":        {"Entrá para publicar.", "Log in to post."},
	"entra_responder":   {"Entrá para responder.", "Log in to reply."},

	// Publicación
	"guardar":         {"Guardar", "Save"},
	"sacar_guardados": {"Sacar de guardados", "Unsave"},
	"responder":       {"Responder", "Reply"},
	"reportar":        {"Reportar", "Report"},
	"borrar":          {"Borrar", "Delete"},
	"vos":             {"(vos)", "(you)"},
	"nuevo":           {"nuevo", "new"},
	"en_revision":     {"En revisión: por ahora solo lo ves vos.", "Under review: only you can see it for now."},
	"respuestas_de":   {"Respuestas:", "Replies:"},
	"hay_nuevos":      {"Hay mensajes nuevos (se agregaron solos). Total: %d.", "New messages arrived (added automatically). Total: %d."},
	"sigue":           {"(sigue)", "(continues)"},
	"viene":           {"(viene de la página anterior)", "(continued from previous page)"},
	"no_acepta":       {"Esta publicación ya no acepta respuestas.", "This thread no longer accepts replies."},
	"cita_fuera":      {">>%d está en otra publicación: abriéndola…", ">>%d is in another thread: opening it…"},
	"cita_no_hay":     {"Este mensaje no cita a nadie.", "This message doesn't quote anyone."},

	// Formularios
	"asunto":         {"Asunto", "Subject"},
	"mensaje":        {"Mensaje", "Message"},
	"tablon":         {"Tablón", "Board"},
	"vista_previa":   {"Vista previa", "Preview"},
	"editar":         {"Editar", "Edit"},
	"cancelar":       {"Cancelar", "Cancel"},
	"revisando":      {"Revisando…", "Reviewing…"},
	"sage":           {"sage: responder sin subir la publicación", "sage: reply without bumping the thread"},
	"ayuda_codigos":  {"> al inicio de una línea: cita · >>123: referencia a un post · [spoiler]texto[/spoiler]", "> at line start: quote · >>123: link to a post · [spoiler]text[/spoiler]"},
	"previa_nota":    {"Vista previa: todavía no se publicó.", "Preview: not published yet."},
	"en":             {"En: %s", "In: %s"},
	"elegi_tablon":   {"Elegí en qué tablón publicarlo.", "Choose a board to post in."},
	"falta_asunto":   {"Falta el asunto.", "The subject is missing."},
	"vacio":          {"El mensaje está vacío.", "The message is empty."},
	"largo_asunto":   {"El asunto puede tener hasta %d caracteres.", "The subject can be up to %d characters."},
	"largo_cuerpo":   {"El mensaje puede tener hasta %d caracteres.", "The message can be up to %d characters."},
	"borrador":       {"Recuperé tu borrador.", "Restored your draft."},
	"en_cola":        {"Tu mensaje quedó en revisión. Mientras tanto solo lo ves vos.", "Your message is under review. Only you can see it for now."},
	"publicado":      {"Publicado.", "Published."},
	"atajo_publicar": {"Ctrl+Enter o Ctrl+S publica", "Ctrl+Enter or Ctrl+S posts"},

	// Reportar / borrar
	"motivo":         {"Motivo", "Reason"},
	"enviar_reporte": {"Enviar reporte", "Send report"},
	"reportado":      {"Gracias por el reporte. Lo va a revisar un moderador.", "Thanks for the report. A moderator will review it."},
	"reportar_ayuda": {"Los mensajes reportados por varias personas se ocultan hasta que los mire un moderador.", "Messages reported by several people are hidden until a moderator reviews them."},
	"borrar_ayuda":   {"En su lugar queda \"Eliminado por su autor\".", "It will say \"Deleted by its author\" instead."},
	"borrar_op":      {"En su lugar queda \"Eliminado por su autor\" y la publicación pasa a llamarse \"(eliminada)\".", "It will say \"Deleted by its author\" and the thread will be renamed \"(deleted)\"."},
	"no_deshacer":    {"No se puede deshacer.", "This can't be undone."},
	"si_borrar":      {"Sí, borrar", "Yes, delete"},
	"volver":         {"Volver", "Back"},
	"borrado":        {"Tu mensaje quedó borrado.", "Your message was deleted."},

	// Respuestas / guardados
	"respuestas_ayuda":   {"Comentarios en las publicaciones que abriste o guardaste, y mensajes que te citan con >>. Solo dentro del sitio: no mandamos mails ni notificaciones.", "Comments in threads you started or saved, and messages that quote you with >>. Only inside the site: no emails or notifications."},
	"sin_respuestas":     {"Todavía no hay respuestas.", "No replies yet."},
	"donde_participaste": {"Donde participaste", "Where you posted"},
	"sin_participar":     {"Todavía no publicaste nada.", "You haven't posted anything yet."},
	"solo_vos":           {"Solo lo ves vos. En cada publicación, tus mensajes aparecen marcados con \"(vos)\".", "Only you can see this. In each thread, your messages are marked \"(you)\"."},
	"guardados_ayuda":    {"Publicaciones que guardaste para leer después. Solo las ves vos. Cuando alguien comenta en una, te avisa en Respuestas.", "Threads you saved to read later. Only you can see them. When someone comments on one, you're told in Replies."},
	"sin_guardados":      {"Todavía no guardaste nada.", "You haven't saved anything yet."},
	"nueva":              {"nueva", "new"},

	// Búsqueda
	"palabras":     {"Palabras a buscar", "Words to search for"},
	"resultados_1": {"1 resultado, incluido el archivo.", "1 result, archive included."},
	"resultados_n": {"%d resultados, incluido el archivo.", "%d results, archive included."},
	"nada":         {"No encontré nada con eso.", "Nothing found."},

	// Entrar
	"entrar_texto":          {"En el foro nadie ve con qué cuenta entraste: todos los mensajes son pseudoanónimos. De tu cuenta de Google solo se guarda un identificador, para poder suspenderla si no respeta las normas. El correo no se guarda.", "Nobody on the forum can see which account you logged in with: every message is pseudonymous. Only an identifier of your Google account is stored, so it can be suspended if it breaks the rules. Your email isn't stored."},
	"entrar_google":         {"Entrar con Google", "Log in with Google"},
	"entrar_como":           {"Se abrió una ventana de %s con la página de Google del sitio. Entrá con tu cuenta.", "A %s window opened on the site's Google page. Log in with your account."},
	"entrar_cuando":         {"Cuando veas el foro, cerrá esa ventana (o volvé acá y apretá Enter). Recién ahí txt421 lee la sesión.", "Once you see the forum, close that window (or come back here and press Enter). Only then does txt421 read the session."},
	"entrar_ya":             {"ya entré", "I'm logged in"},
	"entrar_leyendo":        {"Cerrando la ventana y leyendo la sesión…", "Closing the window and reading the session…"},
	"entrar_ya_abierto":     {"Ya hay una ventana de txt421 abierta (de un intento anterior): cerrala y probá de nuevo.", "There's already a txt421 browser window open (from an earlier try): close it and try again."},
	"entrar_ok":             {"Listo: entraste.", "Done: you're logged in."},
	"entrar_pegar":          {"Pegar la cookie a mano", "Paste the cookie manually"},
	"entrar_pegar_ayuda":    {"En un navegador donde ya entraste a txt.421.news: herramientas de desarrollo (F12) → Aplicación/Almacenamiento → Cookies → txt.421.news → copiá el valor de «sid».", "In a browser where you're logged in to txt.421.news: developer tools (F12) → Application/Storage → Cookies → txt.421.news → copy the value of «sid»."},
	"entrar_sin_nav":        {"No encontré Chrome, Edge, Brave ni Chromium. Instalá uno o pegá la cookie a mano.", "Couldn't find Chrome, Edge, Brave or Chromium. Install one or paste the cookie manually."},
	"entrar_cerrado":        {"Se cerró la ventana sin que el sitio diera una sesión. Probá de nuevo y esperá a ver el foro antes de cerrarla.", "The window closed before the site gave a session. Try again and wait until you see the forum before closing it."},
	"entrar_invalida":       {"Esa cookie no sirve: el sitio no la reconoce.", "That cookie doesn't work: the site doesn't recognize it."},
	"entrar_google_bloqueo": {"Si Google dice que el navegador no es seguro, usá «Pegar la cookie a mano».", "If Google says the browser isn't secure, use «Paste the cookie manually»."},
	"sesion_vencida":        {"Tu sesión venció o se cerró. Entrá de nuevo.", "Your session expired or was closed. Log in again."},
	"saliste":               {"Saliste.", "Logged out."},
	"confirmar_salir":       {"¿Salir de tu cuenta en txt421?", "Log out of your account in txt421?"},

	// Preferencias
	"tema":           {"Tema", "Theme"},
	"listados":       {"Listados", "Lists"},
	"idioma":         {"Idioma", "Language"},
	"idioma_sistema": {"sistema", "system"},
	"cuenta":         {"Cuenta", "Account"},
	"conectado":      {"Entraste con tu cuenta de Google.", "You're logged in with your Google account."},
	"desconectado":   {"No entraste.", "You're not logged in."},
	"version":        {"Versión %s", "Version %s"},
	"hay_version":    {"Hay una versión nueva: %s. Actualizá desde el menú.", "A new version is available: %s. Update from the menu."},
	"actualizando":   {"Actualizando…", "Updating…"},
	"actualizado":    {"Actualizado a %s. Cerrá y volvé a abrir txt421.", "Updated to %s. Close and reopen txt421."},
	"al_dia":         {"Ya tenés la última versión (%s).", "You already have the latest version (%s)."},

	// Teclas (barra de abajo)
	"k_mover":     {"mover", "move"},
	"k_abrir":     {"abrir", "open"},
	"k_pagina":    {"página", "page"},
	"k_seccion":   {"sección", "section"},
	"k_publicar":  {"publicar", "post"},
	"k_buscar":    {"buscar", "search"},
	"k_ayuda":     {"ayuda", "help"},
	"k_volver":    {"volver", "back"},
	"k_responder": {"responder", "reply"},
	"k_citar":     {"citar", "quote"},
	"k_ir_cita":   {"ir a >>", "go to >>"},
	"k_guardar":   {"guardar", "save"},
	"k_menu":      {"menú", "menu"},
	"k_campo":     {"campo", "field"},
	"k_previa":    {"vista previa", "preview"},
	"k_elegir":    {"elegir", "choose"},
	"k_salir":     {"salir", "quit"},
	"k_vista":     {"vista", "view"},
	"k_tema":      {"tema", "theme"},

	// Ayuda
	"ayuda_titulo": {"Teclas", "Keys"},
	"ayuda_cerrar": {"Cualquier tecla cierra esta ayuda.", "Any key closes this help."},

	// Errores
	"cargando":        {"Cargando…", "Loading…"},
	"error":           {"ERROR: ", "ERROR: "},
	"no_existe":       {"No encontrado: esa página no existe o ya no está disponible.", "Not found: that page doesn't exist or is no longer available."},
	"sin_red":         {"Sin conexión con txt.421.news. Reintentá con r.", "No connection to txt.421.news. Retry with r."},
	"chica":           {"Agrandá la ventana (mínimo %d×%d).", "Make the window bigger (at least %d×%d)."},
	"necesita_entrar": {"Para eso tenés que entrar.", "You need to log in for that."},
}
