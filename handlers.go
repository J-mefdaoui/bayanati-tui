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

// Search and filter functions
func (m *Model) filterReports() {
	if m.SearchQuery == "" {
		m.FilteredReports = m.Reports
	} else {
		filtered := []Report{}
		query := strings.ToLower(m.SearchQuery)

		for _, report := range m.Reports {
			if matchesSearch(report, m.Geotags, query) {
				filtered = append(filtered, report)
			}
		}
		m.FilteredReports = filtered
	}

	// Update pagination
	totalItems := len(m.FilteredReports)
	m.TotalPages = (totalItems + m.ItemsPerPage - 1) / m.ItemsPerPage
	if m.TotalPages == 0 {
		m.TotalPages = 1
	}
	if m.CurrentPage > m.TotalPages {
		m.CurrentPage = m.TotalPages
	}
	if m.CurrentPage < 1 {
		m.CurrentPage = 1
	}
}

func matchesSearch(report Report, geotags []GeoTags, query string) bool {
	// Search by Report ID
	if strings.Contains(strings.ToLower(report.ID), query) {
		return true
	}

	// Search by date (YYYY-MM-DD)
	if len(query) == 10 && query[4] == '-' && query[7] == '-' {
		if strings.Contains(report.Created[:10], query) {
			return true
		}
	}

	// Search by status
	if query == "verified" && report.Verified {
		return true
	}
	if query == "unverified" && !report.Verified {
		return true
	}

	// Search by geotag properties (governorate, category)
	for _, refID := range report.Reference {
		for _, gt := range geotags {
			if gt.ID == refID {
				if strings.Contains(strings.ToLower(gt.Governorate), query) {
					return true
				}
				if strings.Contains(strings.ToLower(gt.Category), query) {
					return true
				}
			}
		}
	}

	return false
}

func (m *Model) currentPageReports() []Report {
	start := (m.CurrentPage - 1) * m.ItemsPerPage
	end := start + m.ItemsPerPage
	if end > len(m.FilteredReports) {
		end = len(m.FilteredReports)
	}
	if start > len(m.FilteredReports) {
		return []Report{}
	}
	return m.FilteredReports[start:end]
}

func (m *Model) nextPage() {
	if m.CurrentPage < m.TotalPages {
		m.CurrentPage++
	}
}

func (m *Model) prevPage() {
	if m.CurrentPage > 1 {
		m.CurrentPage--
	}
}

func (m *Model) firstPage() {
	m.CurrentPage = 1
}

func (m *Model) lastPage() {
	m.CurrentPage = m.TotalPages
}

func (m *Model) clearSearch() {
	m.SearchQuery = ""
	m.SearchFocused = false
	m.CurrentPage = 1
	m.filterReports()
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
		if m.SearchFocused {
        switch msg.String() {
        case "esc":
            m.clearSearch()
        case "enter":
            m.SearchFocused = false
            m.filterReports()
        case "backspace":
            if len(m.SearchQuery) > 0 {
                m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
                m.filterReports()
                m.CurrentPage = 1
            }
        default:
            char := msg.String()
            if len(char) == 1 && char >= " " && char <= "~" {
                m.SearchQuery += char
                m.filterReports()
                m.CurrentPage = 1
            }
        }
        return m, nil
    }

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.ActivePanel = 1 - m.ActivePanel
		case "up", "k":
			if m.ActivePanel == 0 && len(m.FilteredReports) > 0 {
				pageReports := m.currentPageReports()
				if len(pageReports) > 0 {
					m.SelectedReportIdx = (m.SelectedReportIdx - 1 + len(pageReports)) % len(pageReports)
					m.CurrentReport = &pageReports[m.SelectedReportIdx]
					m.CurrentGeotag = nil
					filterGeotagsForCurrentReport(m)
					m.SelectedGeotagIdx = 0
					if len(m.FilteredGeotags) > 0 {
						m.CurrentGeotag = &m.FilteredGeotags[0]
					}
				}
			} else if m.ActivePanel == 1 && len(m.FilteredGeotags) > 0 {
				m.SelectedGeotagIdx = (m.SelectedGeotagIdx - 1 + len(m.FilteredGeotags)) % len(m.FilteredGeotags)
				m.CurrentGeotag = &m.FilteredGeotags[m.SelectedGeotagIdx]
				m.CurrentReport = nil
			}
		case "down", "j":
			if m.ActivePanel == 0 && len(m.FilteredReports) > 0 {
				pageReports := m.currentPageReports()
				if len(pageReports) > 0 {
					m.SelectedReportIdx = (m.SelectedReportIdx + 1) % len(pageReports)
					m.CurrentReport = &pageReports[m.SelectedReportIdx]
					m.CurrentGeotag = nil
					filterGeotagsForCurrentReport(m)
					m.SelectedGeotagIdx = 0
					if len(m.FilteredGeotags) > 0 {
						m.CurrentGeotag = &m.FilteredGeotags[0]
					}
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
		case "/":
			m.SearchFocused = true
			return m, nil
		case "esc":
			if m.SearchFocused {
				m.clearSearch()
			}
			return m, nil
		case "enter":
			if m.SearchFocused {
				m.SearchFocused = false
				m.filterReports()
			}
			return m, nil
		case "backspace":
			if m.SearchFocused && len(m.SearchQuery) > 0 {
				m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
				m.filterReports()
				m.CurrentPage = 1
			}
			return m, nil
		case "left", "h":
			if !m.SearchFocused {
				m.prevPage()
			}
			return m, nil
		case "right", "l":
			if !m.SearchFocused {
				m.nextPage()
			}
			return m, nil
		case "g":
			if !m.SearchFocused {
				m.firstPage()
			}
			return m, nil
		case "G":
			if !m.SearchFocused {
				m.lastPage()
			}
			return m, nil
		default:
			// Handle typing in search bar when focused
			if m.SearchFocused {
				char := msg.String()
				if len(char) == 1 && char >= " " && char <= "~" {
					m.SearchQuery += char
					m.filterReports()
					m.CurrentPage = 1
				}
			}
		}

	case FetchReportsMsg:
		if msg.Err == nil {
			m.Reports = msg.Reports
			m.filterReports()
			if len(m.FilteredReports) > 0 {
				pageReports := m.currentPageReports()
				if len(pageReports) > 0 {
					m.CurrentReport = &pageReports[0]
					filterGeotagsForCurrentReport(m)
					if len(m.FilteredGeotags) > 0 {
						m.CurrentGeotag = &m.FilteredGeotags[0]
					}
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
			m.filterReports()
			if len(m.FilteredReports) > 0 && m.CurrentReport == nil {
				pageReports := m.currentPageReports()
				if len(pageReports) > 0 {
					m.CurrentReport = &pageReports[0]
					filterGeotagsForCurrentReport(m)
					if len(m.FilteredGeotags) > 0 {
						m.CurrentGeotag = &m.FilteredGeotags[0]
					}
				}
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
