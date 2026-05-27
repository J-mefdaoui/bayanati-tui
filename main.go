package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

// Wrapper that implements tea.Model
type teaModel struct {
	*Model
}

func (m teaModel) Init() tea.Cmd {
	return nil
}

func (m teaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := update(m.Model, msg)
	return teaModel{newModel}, cmd
}

func (m teaModel) View() string {
	return view(*m.Model)
}

func main() {
	initialModel := InitialModel()
	defer cleanupCache(&initialModel)

	p := tea.NewProgram(teaModel{&initialModel}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
