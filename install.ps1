# Instala txt421 en Windows (PowerShell):
#   irm https://raw.githubusercontent.com/piojosso/txt421/main/install.ps1 | iex
#
# Baja el ejecutable de la última versión, verifica su checksum, lo deja en
# %LOCALAPPDATA%\Programs\txt421 y agrega esa carpeta al PATH del usuario.
# Todo va en un bloque: con "irm | iex" corre en tu sesión de PowerShell, y así no te cambia
# preferencias ni te deja variables.
& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'   # sin la barra, Invoke-WebRequest es mucho más rápido
  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

  $repo = 'piojosso/txt421'
  $dir = if ($env:TXT421_DIR) { $env:TXT421_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\txt421' }

  $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'ARM64' { 'arm64' }
    'AMD64' { 'amd64' }
    default {
      if ($env:PROCESSOR_ARCHITEW6432 -eq 'AMD64') { 'amd64' }
      else { throw "txt421: no hay versión para $($env:PROCESSOR_ARCHITECTURE)" }
    }
  }

  $archivo = "txt421_windows_$arch.zip"
  $base = "https://github.com/$repo/releases/latest/download"
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("txt421-" + [Guid]::NewGuid())
  New-Item -ItemType Directory -Path $tmp | Out-Null
  try {
    Write-Host "Bajando txt421 (windows/$arch)..."
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$archivo" -OutFile (Join-Path $tmp $archivo)
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

    $linea = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match "\s$([regex]::Escape($archivo))$" } | Select-Object -First 1
    if (-not $linea) { throw "txt421: $archivo no figura en checksums.txt" }
    $esperado = ($linea -split '\s+')[0].ToLower()
    $real = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archivo)).Hash.ToLower()
    if ($esperado -ne $real) { throw 'txt421: la descarga no coincide con su checksum; no se instaló nada' }

    Expand-Archive -Path (Join-Path $tmp $archivo) -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $exe = Join-Path $dir 'txt421.exe'
    # Un .exe abierto no se puede pisar, pero sí renombrar.
    if (Test-Path $exe) {
      Remove-Item "$exe.old" -Force -ErrorAction SilentlyContinue
      try { Remove-Item $exe -Force } catch { Rename-Item $exe "$exe.old" }
    }
    Move-Item (Join-Path $tmp 'txt421.exe') $exe -Force
    Unblock-File $exe -ErrorAction SilentlyContinue
  } finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
  }

  $version = & $exe version
  Write-Host "Instalado: $version en $exe"

  # El PATH del usuario se lee y se escribe sin expandir (%USERPROFILE%… quedan como estaban).
  $clave = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
  $pathUsuario = $clave.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
  $partes = @($pathUsuario -split ';' | Where-Object { $_ })
  if ($partes -notcontains $dir) {
    $clave.SetValue('Path', (($partes + $dir) -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)
    # Avisar a Windows que cambió el entorno (lo hace SetEnvironmentVariable, con una variable descartable).
    [Environment]::SetEnvironmentVariable('TXT421_INSTALANDO', '1', 'User')
    [Environment]::SetEnvironmentVariable('TXT421_INSTALANDO', $null, 'User')
    Write-Host "Agregué $dir al PATH."
  }
  $clave.Close()
  if (($env:Path -split ';') -notcontains $dir) { $env:Path = "$env:Path;$dir" }
  Write-Host 'Listo. Escribí: txt421   (en una ventana nueva de la terminal, si esta no lo encuentra)'
  Write-Host 'Se ve mejor en Windows Terminal.'
}
