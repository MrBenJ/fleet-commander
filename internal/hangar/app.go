package hangar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type BrowserOpener func(string)

type App struct {
	cfg         Config
	openBrowser BrowserOpener
	newProgram  func(tea.Model) teaProgram
}

type teaProgram interface {
	Run() (tea.Model, error)
	Send(msg tea.Msg)
}

type teaProgramAdapter struct {
	*tea.Program
}

func NewApp(cfg Config, openBrowser BrowserOpener) *App {
	if openBrowser == nil {
		openBrowser = func(string) {}
	}
	return &App{
		cfg:         cfg,
		openBrowser: openBrowser,
		newProgram: func(model tea.Model) teaProgram {
			return teaProgramAdapter{Program: tea.NewProgram(model)}
		},
	}
}

func (a *App) Run(ctx context.Context, noOpen bool) error {
	srv := NewServer(a.cfg)
	url := fmt.Sprintf("http://localhost:%d", a.cfg.Port)
	if a.cfg.ControlSquadron != "" {
		url += "?squadron=" + a.cfg.ControlSquadron
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()

	if !noOpen {
		a.openBrowser(url)
	}

	p := a.newProgram(NewTUIModel(url))
	go func() {
		for msg := range srv.LogCh {
			p.Send(LogMsg{Message: msg})
		}
	}()

	_, runErr := p.Run()
	cancel()
	serverErr := waitForServer(errCh)
	if runErr != nil {
		return runErr
	}
	return serverErr
}

func waitForServer(errCh <-chan error) error {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-timer.C:
		return fmt.Errorf("hangar server did not shut down within %s", 5*time.Second)
	}
}
