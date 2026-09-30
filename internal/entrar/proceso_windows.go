//go:build windows

package entrar

import (
	"os/exec"
	"strconv"
	"syscall"
)

// CREATE_NEW_PROCESS_GROUP: un Ctrl+C en la consola no le llega al navegador.
func prepararProceso(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
}

// pedirCierre: taskkill sin /F le manda a la ventana el pedido de cerrarse (como la X).
func pedirCierre(cmd *exec.Cmd) {
	k := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid))
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	k.Run()
}
