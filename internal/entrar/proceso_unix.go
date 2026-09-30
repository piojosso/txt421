//go:build !windows

package entrar

import (
	"os/exec"
	"syscall"
)

// El navegador va en su propio grupo de procesos: un Ctrl+C en la terminal no lo mata a medias.
func prepararProceso(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
