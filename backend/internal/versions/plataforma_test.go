package versions

import (
	"runtime"
	"testing"
)

// skipOnWindows pula testes que dependem de algo que o Windows não tem:
// permissões Unix (chmod 0o000 ou 0o555 numa pasta não impede ler nem gravar)
// e links simbólicos (exigem administrador ou o Modo de Desenvolvedor).
func skipOnWindows(t *testing.T, why string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no Windows: " + why)
	}
}
