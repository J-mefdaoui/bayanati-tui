package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func filterGeotagsForCurrentReport(m *Model) {
	if m.CurrentReport == nil || len(m.Geotags) == 0 {
		m.FilteredGeotags = []GeoTags{}
		return
	}

	referencedIDs := make(map[string]bool)
	for _, refID := range m.CurrentReport.Reference {
		referencedIDs[strings.TrimSpace(refID)] = true
	}

	filtered := []GeoTags{}
	for _, gt := range m.Geotags {
		if referencedIDs[gt.ID] {
			filtered = append(filtered, gt)
		}
	}
	m.FilteredGeotags = filtered
}

func cleanupCache(m *Model) {
	for _, filepath := range m.DownloadedFiles {
		os.Remove(filepath)
	}
}

func updateLogin(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.Email != "" && m.Password != "" {
				return m, m.login()
			}
		case "tab":
			m.FocusedField = 1 - m.FocusedField
		case "up", "k":
			m.FocusedField = 0
		case "down", "j":
			m.FocusedField = 1
		case "backspace":
			if m.FocusedField == 0 && len(m.Email) > 0 {
				m.Email = m.Email[:len(m.Email)-1]
			} else if m.FocusedField == 1 && len(m.Password) > 0 {
				m.Password = m.Password[:len(m.Password)-1]
			}
		default:
			char := msg.String()
			if len(char) == 1 && char >= " " && char <= "~" {
				if m.FocusedField == 0 {
					m.Email += char
				} else if m.FocusedField == 1 {
					m.Password += char
				}
			}
		}
	case LoginMsg:
		m.Loading = false
		if msg.Success {
			m.Token = msg.Token
			m.Governorate = msg.Governorate
			m.LoggedIn = true
			return m, tea.Batch(m.fetchReports(), m.fetchGeoTags())
		} else {
			m.LoginError = msg.Err.Error()
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func updateMain(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	case TickMsg:
		if m.DownloadingPDF {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return TickMsg{}
			})
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.ActivePanel = 1 - m.ActivePanel
		case "up", "k":
			if m.ActivePanel == 0 && len(m.Reports) > 0 {
				m.SelectedReportIdx = (m.SelectedReportIdx - 1 + len(m.Reports)) % len(m.Reports)
				m.CurrentReport = &m.Reports[m.SelectedReportIdx]
				m.CurrentGeotag = nil
				filterGeotagsForCurrentReport(m)
				m.SelectedGeotagIdx = 0
				if len(m.FilteredGeotags) > 0 {
					m.CurrentGeotag = &m.FilteredGeotags[0]
				}
			} else if m.ActivePanel == 1 && len(m.FilteredGeotags) > 0 {
				m.SelectedGeotagIdx = (m.SelectedGeotagIdx - 1 + len(m.FilteredGeotags)) % len(m.FilteredGeotags)
				m.CurrentGeotag = &m.FilteredGeotags[m.SelectedGeotagIdx]
				m.CurrentReport = nil
			}
		case "down", "j":
			if m.ActivePanel == 0 && len(m.Reports) > 0 {
				m.SelectedReportIdx = (m.SelectedReportIdx + 1) % len(m.Reports)
				m.CurrentReport = &m.Reports[m.SelectedReportIdx]
				m.CurrentGeotag = nil
				filterGeotagsForCurrentReport(m)
				m.SelectedGeotagIdx = 0
				if len(m.FilteredGeotags) > 0 {
					m.CurrentGeotag = &m.FilteredGeotags[0]
				}
			} else if m.ActivePanel == 1 && len(m.FilteredGeotags) > 0 {
				m.SelectedGeotagIdx = (m.SelectedGeotagIdx + 1) % len(m.FilteredGeotags)
				m.CurrentGeotag = &m.FilteredGeotags[m.SelectedGeotagIdx]
				m.CurrentReport = nil
			}
		case "v", "V":
			if m.ActivePanel == 0 && m.CurrentReport != nil {
				m.Verifying = true
				m.VerifyingForReportID = m.CurrentReport.ID
				return m, m.updateReport(m.CurrentReport.ID, !m.CurrentReport.Verified)
			}
		case "p", "P":
			if m.CurrentReport != nil && m.CurrentReport.Document != "" {
				if existingPath, exists := m.DownloadedFiles[m.CurrentReport.ID]; exists {
					if _, err := os.Stat(existingPath); err == nil {
						return m, tea.Batch(
							openPDF(existingPath),
							func() tea.Msg {
								return SnackbarMsg{Message: "PDF opened from cache", MsgType: "success"}
							},
						)
					}
				}

				m.DownloadingPDF = true
				m.DownloadingForReportID = m.CurrentReport.ID

				return m, tea.Batch(
					m.downloadPDF(m.CurrentReport.ID, m.CurrentReport.Document),
					tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
						return TickMsg{}
					}),
				)
			}
		case "o", "O":
			if m.ActivePanel == 1 && m.CurrentGeotag != nil {
				return m, openCoordinates(m.CurrentGeotag.Location.Lat, m.CurrentGeotag.Location.Lon)
			}
		}

	case FetchReportsMsg:
		if msg.Err == nil {
			m.Reports = msg.Reports
			if len(m.Reports) > 0 {
				m.CurrentReport = &m.Reports[0]
				filterGeotagsForCurrentReport(m)
				if len(m.FilteredGeotags) > 0 {
					m.CurrentGeotag = &m.FilteredGeotags[0]
				}
			}
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("Loaded %d reports", len(m.Reports)), MsgType: "success"}
			}
		} else {
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("Failed to load reports: %v", msg.Err), MsgType: "error"}
			}
		}

	case FetchGeoTagsMsg:
		if msg.Err == nil {
			m.Geotags = msg.Geotags
			filterGeotagsForCurrentReport(m)
			if len(m.FilteredGeotags) > 0 && m.CurrentGeotag == nil {
				m.CurrentGeotag = &m.FilteredGeotags[0]
			}
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("Loaded %d geotags", len(m.Geotags)), MsgType: "success"}
			}
		} else {
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("Failed to load geotags: %v", msg.Err), MsgType: "error"}
			}
		}

	case UpdateMsg:
		m.Verifying = false
		m.VerifyingForReportID = ""

		if msg.Success {
			status := "verified"
			if !msg.Verified {
				status = "unverified"
			}
			return m, tea.Batch(
				m.fetchReports(),
				func() tea.Msg {
					return SnackbarMsg{Message: fmt.Sprintf("Report batch %s... %s", msg.ID[:8], status), MsgType: "success"}
				},
			)
		} else {
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("Failed to update: %v", msg.Err), MsgType: "error"}
			}
		}

	case PDFMsg:
		m.DownloadingPDF = false
		m.DownloadingForReportID = ""

		if msg.Success {
			if !msg.Cached {
				m.DownloadedFiles[msg.ReportID] = msg.Filepath
			}

			cacheMsg := ""
			if msg.Cached {
				cacheMsg = "(from cache) "
			}
			return m, tea.Batch(
				openPDF(msg.Filepath),
				func() tea.Msg {
					return SnackbarMsg{Message: fmt.Sprintf("PDF opened: %s%s", cacheMsg, msg.Filepath), MsgType: "success"}
				},
			)
		} else {
			return m, func() tea.Msg {
				return SnackbarMsg{Message: fmt.Sprintf("PDF error: %v", msg.Err), MsgType: "error"}
			}
		}

	case SnackbarMsg:
		m.Snackbar = msg.Message
		m.SnackbarType = msg.MsgType
		m.SnackbarExpiry = time.Now().Add(6 * time.Second)
		return m, tea.Tick(6*time.Second, func(t time.Time) tea.Msg {
			return SnackbarMsg{Message: "", MsgType: ""}
		})

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	// Clear snackbar after expiry
	if m.Snackbar != "" && time.Now().After(m.SnackbarExpiry) {
		m.Snackbar = ""
	}

	return m, nil
}

func update(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	if !m.LoggedIn {
		return updateLogin(m, msg)
	}
	return updateMain(m, msg)
}
