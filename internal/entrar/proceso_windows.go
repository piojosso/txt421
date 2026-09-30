//go:build windows

package entrar

import (
	"os/exec"
	"syscall"
)

// CREATE_NEW_PROCESS_GROUP: un Ctrl+C en la consola no le llega al navegador.
func prepararProceso(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
}
