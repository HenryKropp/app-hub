package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/shared/health"
)

// Publish stages, shown while a test or publish is building (MCP F1).
const (
	StageBackup  = "fazendo a cópia de segurança dos dados"
	StageBuild   = "montando o app (instalando as bibliotecas)"
	StageDB      = "preparando o banco do app"
	StageStart   = "ligando o app"
	StageWaiting = "esperando o app responder"
	StageSwap    = "trocando para a versão nova"
)

// Stage is where a building deployment is now. Kept in memory only: after a
// restart the deployment is a leftover anyway.
type Stage struct {
	Name  string
	Since time.Time
}

func (s *Service) setStage(depID int64, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stages == nil {
		s.stages = map[int64]Stage{}
	}
	if name == "" {
		delete(s.stages, depID)
		return
	}
	s.stages[depID] = Stage{Name: name, Since: s.now()}
}

// StageOf is the current stage of a building deployment.
func (s *Service) StageOf(depID int64) (Stage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.stages[depID]
	return st, ok
}

// pollEvery is how often waitUp looks at the app (tests make it shorter).
var pollEvery = 500 * time.Millisecond

// waitUp waits for the app to answer its health path. It stops early when the
// program ends, so a crash at start is reported in seconds, not after the
// whole health timeout.
//
// With checkText, answering is not enough: the page at healthPath must also
// show that text (MCP F4: a quick check of the new version before the swap).
func (s *Service) waitUp(ctx context.Context, cid string, port int, healthPath, checkText string) *failure {
	start := s.now()
	deadline := time.Now().Add(s.opt.HealthTimeout)
	answered, slow := false, false
	for {
		st, err := s.eng.Inspect(ctx, cid)
		if err == nil && st.Exists && !st.Running {
			return s.exitedFailure(ctx, cid, port, st.ExitCode, s.now().Sub(start))
		}
		if err == nil && st.HostPort != 0 {
			url := fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, healthPath)
			if checkText == "" {
				ok, late := health.Probe(ctx, url)
				if ok {
					return nil
				}
				slow = slow || late
			}
			if checkText != "" {
				if body, ok := health.Page(ctx, url); ok {
					answered = true
					if strings.Contains(body, checkText) {
						return nil
					}
				}
			}
		}
		if time.Now().After(deadline) {
			logs, _ := s.eng.Logs(ctx, cid, 60)
			if answered {
				return &failure{msg: fmt.Sprintf("A versão nova abriu, mas a página %s não mostra o texto esperado %q. "+
					"Confira o texto a conferir na ficha ou o que a página mostra (o Ziv lê a página sem rodar o JavaScript).",
					healthPath, checkText), log: logs}
			}
			if slow {
				return &failure{msg: fmt.Sprintf("O programa ligou, mas a página %s demorou mais de %s para responder. "+
					"Deixe a página inicial mais leve (o trabalho pesado pode rodar depois que ela abre).",
					healthPath, seconds(health.AnswerTimeout)), log: logs}
			}
			return &failure{msg: fmt.Sprintf("O programa está ligado há %s, mas não respondeu na porta %d. "+
				"Confira se ele escuta em 0.0.0.0 na porta da variável PORT (e não em 127.0.0.1 ou numa porta fixa).",
				seconds(s.opt.HealthTimeout), port), log: logs}
		}
		select {
		case <-ctx.Done():
			return &failure{msg: "A publicação foi interrompida.", log: ctx.Err().Error()}
		case <-time.After(pollEvery):
		}
	}
}

func (s *Service) exitedFailure(ctx context.Context, cid string, port, code int, after time.Duration) *failure {
	logs, _ := s.eng.Logs(ctx, cid, 60)
	if code == 0 {
		return &failure{msg: fmt.Sprintf("O programa terminou sozinho %s depois de ligar, sem erro, antes de abrir a porta %d. "+
			"Um app com tela precisa continuar ligado, servindo a página.", seconds(after), port), log: logs}
	}
	return &failure{msg: fmt.Sprintf("O programa parou com erro (código %d) %s depois de ligar, antes de abrir a porta %d. "+
		"O motivo está no fim do log.", code, seconds(after), port), log: logs}
}

func seconds(d time.Duration) string {
	n := int(d.Round(time.Second) / time.Second)
	if n <= 1 {
		return "1 s"
	}
	return fmt.Sprintf("%d s", n)
}

// MaskSecrets swaps the app's own secret values (from its Configurações) for
// •••• in a log, so a log can be shown to people and to Claude.
func (s *Service) MaskSecrets(ctx context.Context, appID int64, text string) string {
	if text == "" || s.vault == nil {
		return text
	}
	rows, err := s.q.ListAppConfig(ctx, appID)
	if err != nil {
		return text
	}
	var vals []string
	for _, r := range rows {
		if !r.IsSecret {
			continue
		}
		v, err := s.vault.Open(r.ValueCipher)
		if err == nil && len(v) >= 4 {
			vals = append(vals, v)
		}
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) }) // longest first
	for _, v := range vals {
		text = strings.ReplaceAll(text, v, "••••")
	}
	return text
}
