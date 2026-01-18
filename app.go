package main

import "context"

// App is the bridge bound to the frontend.
type App struct {
	ctx context.Context
	svc *Service
}

// NewApp wires a new Service.
func NewApp() *App {
	return &App{
		svc: NewService(),
	}
}

// startup stores Wails context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// startup stores Wails context.
func (a *App) shutdown(ctx context.Context) {
	a.StopAll()
}

// GetState returns current config, strategies and latest tag info.
func (a *App) GetState() (*State, error) {
	return a.svc.State()
}

// CheckAndUpdate downloads latest release if newer and returns refreshed state.
func (a *App) CheckAndUpdate() (*State, error) {
	return a.svc.CheckAndUpdate()
}

// RunTests executes the official test script (standard mode, all configs) and updates state.
func (a *App) RunTests() (*State, error) {
	return a.svc.RunTests()
}

// RunStrategy starts a selected BAT strategy (non-service, foreground process) and records last run.
func (a *App) RunStrategy(file string) (*State, error) {
	return a.svc.RunStrategy(file)
}

// StopStrategy stops the tracked running strategy, if any.
func (a *App) StopStrategy() (*State, error) {
	if err := a.svc.StopRunning(); err != nil {
		return nil, err
	}
	return a.svc.State()
}

// StopAll is used on shutdown to ensure cleanup.
func (a *App) StopAll() {
	_ = a.svc.StopRunning()
}
