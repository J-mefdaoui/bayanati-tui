package main

import (
	"runtime"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Color scheme
var (
	greenDeep   = lipgloss.Color("#0f3622")
	greenMid    = lipgloss.Color("#1a5a3a")
	greenAccent = lipgloss.Color("#4ADE80")
	red         = lipgloss.Color("#ef4444")
	mutedColor  = lipgloss.Color("#6b8f71")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(greenAccent).
			Background(greenDeep)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenMid).
			Padding(1, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(greenAccent).
			Bold(true)

	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenAccent).
			Padding(1, 1)

	buttonStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Margin(0, 1).
			Background(greenMid).
			Foreground(lipgloss.Color("#e8f5e9"))

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	snackbarStyle = lipgloss.NewStyle().
			Background(greenDeep).
			Foreground(greenAccent).
			Padding(0, 2).
			Margin(1, 0)
)

const PB_URL = "https"

// PocketBase structures
type GeoTags struct {
	ID          string `json:"id"`
	Governorate string `json:"governorate"`
	Image       string `json:"image"`
	Emailed     bool   `json:"emailed"`
	Category    string `json:"category"`
	Location    struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"location"`
	Created string `json:"created"`
}

type Report struct {
	ID        string   `json:"id"`
	Reference []string `json:"reference"`
	Document  string   `json:"document"`
	Emailed   bool     `json:"emailed"`
	Verified  bool     `json:"verified"`
	Created   string   `json:"created"`
}

type snackbarMsg struct {
	message string
	msgType string
}

type tickMsg struct{}

type model struct {
	// Auth
	email        string
	password     string
	token        string
	governorate  string
	loggedIn     bool
	loginError   string
	focusedField int

	// Data
	reports           []Report
	geotags           []GeoTags
	filteredGeotags   []GeoTags
	selectedReportIdx int
	selectedGeotagIdx int
	activePanel       int

	// Selection
	currentReport *Report
	currentGeotag *GeoTags

	// UI state
	loading        bool
	snackbar       string
	snackbarType   string
	snackbarExpiry time.Time
	verifying      bool
	verifyingForReportID string

	// Window dimensions
	width  int
	height int

	// cache feature
	downloadedFiles        map[string]string
	downloadingPDF         bool
	downloadingForReportID string
}

func initialModel() model {
	return model{
		email:             "",
		password:          "",
		loggedIn:          false,
		focusedField:      0,
		activePanel:       0,
		selectedReportIdx: 0,
		selectedGeotagIdx: 0,
		reports:           []Report{},
		geotags:           []GeoTags{},
		filteredGeotags:   []GeoTags{},
		width:             80,
		height:            24,
		downloadedFiles:   make(map[string]string),
	}
}

// Message types
type loginMsg struct {
	success     bool
	err         error
	token       string
	governorate string
}

type fetchReportsMsg struct {
	reports []Report
	err     error
}

type fetchGeoTagsMsg struct {
	geotags []GeoTags
	err     error
}

type updateMsg struct {
	success  bool
	id       string
	verified bool
	err      error
}

type pdfMsg struct {
	success  bool
	reportID string
	filepath string
	err      error
	cached   bool
}

func getTempDir() string {
	if runtime.GOOS == "windows" {
		return os.TempDir()
	}
	return "/tmp"
}

// API calls
func (m *model) login() tea.Cmd {
	m.loading = true
	m.loginError = ""
	return func() tea.Msg {
		data := strings.NewReader(fmt.Sprintf(`{"identity":"%s","password":"%s"}`, m.email, m.password))
		req, _ := http.NewRequest("POST", PB_URL+"/api/collections/users/auth-with-password", data)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return loginMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return loginMsg{err: fmt.Errorf("login failed: %s", string(body))}
		}

		var result struct {
			Token string `json:"token"`
			Record struct {
				Governorate string `json:"governorate"`
			} `json:"record"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		return loginMsg{success: true, token: result.Token, governorate: result.Record.Governorate}
	}
}

func (m *model) fetchReports() tea.Cmd {
	return func() tea.Msg {
		req, _ := http.NewRequest("GET", PB_URL+"/api/collections/Reports/records?sort=-created", nil)
		req.Header.Set("Authorization", m.token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fetchReportsMsg{err: err}
		}
		defer resp.Body.Close()

		var result struct {
			Items []Report `json:"items"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		return fetchReportsMsg{reports: result.Items}
	}
}

func (m *model) fetchGeoTags() tea.Cmd {
	return func() tea.Msg {
		var allGeotags []GeoTags
		page := 1

		for {
			url := fmt.Sprintf("%s/api/collections/GeoTags/records?sort=-created&perPage=100&page=%d", PB_URL, page)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", m.token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return fetchGeoTagsMsg{err: err}
			}

			var result struct {
				Items      []GeoTags `json:"items"`
				Page       int       `json:"page"`
				PerPage    int       `json:"perPage"`
				TotalItems int       `json:"totalItems"`
				TotalPages int       `json:"totalPages"`
			}
			json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()

			allGeotags = append(allGeotags, result.Items...)

			if page >= result.TotalPages {
				break
			}
			page++
		}

		return fetchGeoTagsMsg{geotags: allGeotags}
	}
}

func (m *model) updateReport(reportID string, verified bool) tea.Cmd {
	return func() tea.Msg {
		data := strings.NewReader(fmt.Sprintf(`{"verified":%v}`, verified))
		req, _ := http.NewRequest("PATCH", PB_URL+"/api/collections/Reports/records/"+reportID, data)
		req.Header.Set("Authorization", m.token)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return updateMsg{err: err}
		}
		defer resp.Body.Close()

		return updateMsg{success: true, id: reportID, verified: verified}
	}
}

func (m *model) cleanupCache() {
	for _, filepath := range m.downloadedFiles {
		os.Remove(filepath)
	}
}

func (m *model) downloadPDF(reportID, filename string) tea.Cmd {
	if existingPath, exists := m.downloadedFiles[reportID]; exists {
		if _, err := os.Stat(existingPath); err == nil {
			return func() tea.Msg {
				return pdfMsg{success: true, reportID: reportID, filepath: existingPath, cached: true}
			}
		} else {
			delete(m.downloadedFiles, reportID)
		}
	}

	return func() tea.Msg {
		fileURL := fmt.Sprintf("%s/api/files/Reports/%s/%s", PB_URL, reportID, filename)

		req, err := http.NewRequest("GET", fileURL, nil)
		if err != nil {
			return pdfMsg{err: fmt.Errorf("failed to create request: %v", err), reportID: reportID}
		}
		req.Header.Set("Authorization", m.token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return pdfMsg{err: fmt.Errorf("failed to download: %v", err), reportID: reportID}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return pdfMsg{err: fmt.Errorf("PDF download failed (status %d): %s", resp.StatusCode, string(body)), reportID: reportID}
		}

		timestamp := time.Now().Unix()
		safeFilename := strings.ReplaceAll(filename, "/", "_")
		safeFilename = strings.ReplaceAll(safeFilename, "\\", "_")
		outFile := fmt.Sprintf("%s/bayanati_%s_%d_%s", getTempDir(), reportID[:8], timestamp, safeFilename)

		out, err := os.Create(outFile)
		if err != nil {
			return pdfMsg{err: fmt.Errorf("failed to create file: %v", err), reportID: reportID}
		}
		defer out.Close()

		written, err := io.Copy(out, resp.Body)
		if err != nil {
			return pdfMsg{err: fmt.Errorf("failed to save PDF: %v", err), reportID: reportID}
		}

		if written == 0 {
			return pdfMsg{err: fmt.Errorf("downloaded PDF is empty"), reportID: reportID}
		}

		m.downloadedFiles[reportID] = outFile

		return pdfMsg{success: true, reportID: reportID, filepath: outFile, cached: false}
	}
}

func (m *model) openPDF(filepath string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			winPath := filepath
			cmd = exec.Command("cmd", "/c", "start", "", winPath)
			if err := cmd.Run(); err != nil {
				cmd = exec.Command("explorer", winPath)
				if err := cmd.Run(); err != nil {
					return snackbarMsg{message: fmt.Sprintf("Failed to open PDF: %v", err), msgType: "error"}
				}
			}
			return snackbarMsg{message: "PDF opened with default viewer", msgType: "success"}
		case "darwin":
			cmd = exec.Command("open", filepath)
		default:
			cmd = exec.Command("xdg-open", filepath)
		}

		if err := cmd.Run(); err != nil {
			return snackbarMsg{message: fmt.Sprintf("Failed to open PDF: %v", err), msgType: "error"}
		}
		return snackbarMsg{message: "PDF opened with default viewer", msgType: "success"}
	}
}

func (m *model) openCoordinates(lat, lon float64) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("https://www.openstreetmap.org/?mlat=%f&mlon=%f&zoom=16#map=16/%f/%f", lat, lon, lat, lon)

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", strings.ReplaceAll(url, "&", "^&"))
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}

		if err := cmd.Run(); err != nil {
			return snackbarMsg{message: fmt.Sprintf("Failed to open map: %v", err), msgType: "error"}
		}
		return snackbarMsg{message: fmt.Sprintf("Opening coordinates: %.6f, %.6f", lat, lon), msgType: "info"}
	}
}

func (m *model) filterGeotagsForCurrentReport() {
	if m.currentReport == nil || len(m.geotags) == 0 {
		m.filteredGeotags = []GeoTags{}
		return
	}

	referencedIDs := make(map[string]bool)
	for _, refID := range m.currentReport.Reference {
		referencedIDs[strings.TrimSpace(refID)] = true
	}

	filtered := []GeoTags{}
	for _, gt := range m.geotags {
		if referencedIDs[gt.ID] {
			filtered = append(filtered, gt)
		}
	}
	m.filteredGeotags = filtered
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.email != "" && m.password != "" {
				return m, m.login()
			}
		case "tab":
			m.focusedField = 1 - m.focusedField
		case "up", "k":
			m.focusedField = 0
		case "down", "j":
			m.focusedField = 1
		case "backspace":
			if m.focusedField == 0 && len(m.email) > 0 {
				m.email = m.email[:len(m.email)-1]
			} else if m.focusedField == 1 && len(m.password) > 0 {
				m.password = m.password[:len(m.password)-1]
			}
		default:
			char := msg.String()
			if len(char) == 1 && char >= " " && char <= "~" {
				if m.focusedField == 0 {
					m.email += char
				} else if m.focusedField == 1 {
					m.password += char
				}
			}
		}
	case loginMsg:
		m.loading = false
		if msg.success {
			m.token = msg.token
			m.governorate = msg.governorate
			m.loggedIn = true
			return m, tea.Batch(m.fetchReports(), m.fetchGeoTags())
		} else {
			m.loginError = msg.err.Error()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *model) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		if m.downloadingPDF {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			})
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.activePanel = 1 - m.activePanel
		case "up", "k":
			if m.activePanel == 0 && len(m.reports) > 0 {
				m.selectedReportIdx = (m.selectedReportIdx - 1 + len(m.reports)) % len(m.reports)
				m.currentReport = &m.reports[m.selectedReportIdx]
				m.currentGeotag = nil
				m.filterGeotagsForCurrentReport()
				m.selectedGeotagIdx = 0
				if len(m.filteredGeotags) > 0 {
					m.currentGeotag = &m.filteredGeotags[0]
				}
			} else if m.activePanel == 1 && len(m.filteredGeotags) > 0 {
				m.selectedGeotagIdx = (m.selectedGeotagIdx - 1 + len(m.filteredGeotags)) % len(m.filteredGeotags)
				m.currentGeotag = &m.filteredGeotags[m.selectedGeotagIdx]
				m.currentReport = nil
			}
		case "down", "j":
			if m.activePanel == 0 && len(m.reports) > 0 {
				m.selectedReportIdx = (m.selectedReportIdx + 1) % len(m.reports)
				m.currentReport = &m.reports[m.selectedReportIdx]
				m.currentGeotag = nil
				m.filterGeotagsForCurrentReport()
				m.selectedGeotagIdx = 0
				if len(m.filteredGeotags) > 0 {
					m.currentGeotag = &m.filteredGeotags[0]
				}
			} else if m.activePanel == 1 && len(m.filteredGeotags) > 0 {
				m.selectedGeotagIdx = (m.selectedGeotagIdx + 1) % len(m.filteredGeotags)
				m.currentGeotag = &m.filteredGeotags[m.selectedGeotagIdx]
				m.currentReport = nil
			}
		case "v", "V":
			if m.activePanel == 0 && m.currentReport != nil {
				m.verifying = true
				m.verifyingForReportID = m.currentReport.ID
				return m, m.updateReport(m.currentReport.ID, !m.currentReport.Verified)
			}
		case "p", "P":
			if m.currentReport != nil && m.currentReport.Document != "" {
				if existingPath, exists := m.downloadedFiles[m.currentReport.ID]; exists {
					if _, err := os.Stat(existingPath); err == nil {
						return m, tea.Batch(
							m.openPDF(existingPath),
							func() tea.Msg {
								return snackbarMsg{message: "PDF opened from cache", msgType: "success"}
							},
						)
					}
				}

				m.downloadingPDF = true
				m.downloadingForReportID = m.currentReport.ID

				return m, tea.Batch(
					m.downloadPDF(m.currentReport.ID, m.currentReport.Document),
					tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
						return tickMsg{}
					}),
				)
			}
		case "o", "O":
			if m.activePanel == 1 && m.currentGeotag != nil {
				return m, m.openCoordinates(m.currentGeotag.Location.Lat, m.currentGeotag.Location.Lon)
			}
		}

	case fetchReportsMsg:
		if msg.err == nil {
			m.reports = msg.reports
			if len(m.reports) > 0 {
				m.currentReport = &m.reports[0]
				m.filterGeotagsForCurrentReport()
				if len(m.filteredGeotags) > 0 {
					m.currentGeotag = &m.filteredGeotags[0]
				}
			}
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("Loaded %d reports", len(m.reports)), msgType: "success"}
			}
		} else {
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("Failed to load reports: %v", msg.err), msgType: "error"}
			}
		}

	case fetchGeoTagsMsg:
		if msg.err == nil {
			m.geotags = msg.geotags
			m.filterGeotagsForCurrentReport()
			if len(m.filteredGeotags) > 0 && m.currentGeotag == nil {
				m.currentGeotag = &m.filteredGeotags[0]
			}
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("Loaded %d geotags", len(m.geotags)), msgType: "success"}
			}
		} else {
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("Failed to load geotags: %v", msg.err), msgType: "error"}
			}
		}

	case updateMsg:
		m.verifying = false
		m.verifyingForReportID = ""

		if msg.success {
			status := "verified"
			if !msg.verified {
				status = "unverified"
			}
			return m, tea.Batch(
				m.fetchReports(),
				func() tea.Msg {
					return snackbarMsg{message: fmt.Sprintf("Report batch %s... %s", msg.id[:8], status), msgType: "success"}
				},
			)
		} else {
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("Failed to update: %v", msg.err), msgType: "error"}
			}
		}

	case pdfMsg:
		m.downloadingPDF = false
		m.downloadingForReportID = ""

		if msg.success {
			if !msg.cached {
				m.downloadedFiles[msg.reportID] = msg.filepath
			}

			cacheMsg := ""
			if msg.cached {
				cacheMsg = "(from cache) "
			}
			return m, tea.Batch(
				m.openPDF(msg.filepath),
				func() tea.Msg {
					return snackbarMsg{message: fmt.Sprintf("PDF opened: %s%s", cacheMsg, msg.filepath), msgType: "success"}
				},
			)
		} else {
			return m, func() tea.Msg {
				return snackbarMsg{message: fmt.Sprintf("PDF error: %v", msg.err), msgType: "error"}
			}
		}

	case snackbarMsg:
		m.snackbar = msg.message
		m.snackbarType = msg.msgType
		m.snackbarExpiry = time.Now().Add(6 * time.Second)
		return m, tea.Tick(6*time.Second, func(t time.Time) tea.Msg {
			return snackbarMsg{message: "", msgType: ""}
		})

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	if m.snackbar != "" && time.Now().After(m.snackbarExpiry) {
		m.snackbar = ""
	}

	return m, nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.loggedIn {
		return m.updateLogin(msg)
	}
	return m.updateMain(msg)
}

func (m model) renderLoginScreen() string {
	logoLines := []string{
		"                    ....                    │ ",
		"                    x. o                    │",
		"                    ;...                    │",
		"                    .doc   ...              │",
		"                    xxol  ,.  .             │",
		"                    xxol  :l;,              │",
		"              o  ;  xxol  :lc:.             │",
		"              ,.'c  xxol  :lc:.             │",
		"              oocc  xxol  :lc:.      .      │",
		"              oocc  ihsen :lc:.      ;;.    │",
		"        '.    .           :lc:.     ,;;;.   │",
		"        .;;;;;;;;;;;,''..   .:.    .;;,;;   │",
		"         ';;;ayoub;;;;;;;;;..      ,;;.;;   │",
		"          ,;;;;;,;;;;;;;;;;;;;.    ,;;.;;   │",
		"           ,;;;;;,'. ;;;;;;;;;;;.   ;..;    │",
		"            .;;;;;;;'...    ;;;;;,  . '     │",
		"              ';;;;;;;;;;'....              │",
		"                 ;;;;;;;;;;;;;;,,.          │",
		"                    .;;;;;;;;;.             │",
	}

	topColor := lipgloss.NewStyle().Foreground(greenAccent)
	middleColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	bottomColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#16A34A"))

	var coloredLines []string
	for i, line := range logoLines {
		switch {
		case i < 6:
			coloredLines = append(coloredLines, topColor.Render(line))
		case i < 12:
			coloredLines = append(coloredLines, middleColor.Render(line))
		default:
			coloredLines = append(coloredLines, bottomColor.Render(line))
		}
	}

	logo := strings.Join(coloredLines, "\n")
	coloredLogo := lipgloss.NewStyle().Render(logo)

	logoWidth := 35
	formWidth := m.width - logoWidth - 10
	if formWidth < 40 {
		formWidth = 40
	}

	loginForm := titleStyle.Width(formWidth).Render("Bayanati Municipality Portal") + "\n\n"

	if m.loginError != "" {
		loginForm += errorStyle.Render("Error: "+m.loginError) + "\n\n"
	}

	emailField := "Email: " + m.email
	if m.focusedField == 0 {
		emailField = selectedStyle.Render("> " + emailField + " <")
	} else {
		emailField = "  " + emailField
	}

	passwordDisplay := strings.Repeat("*", len(m.password))
	passwordField := "Password: " + passwordDisplay
	if m.focusedField == 1 {
		passwordField = selectedStyle.Render("> " + passwordField + " <")
	} else {
		passwordField = "  " + passwordField
	}

	loginForm += emailField + "\n" + passwordField + "\n\n"
	loginForm += lipgloss.NewStyle().Foreground(mutedColor).Render("[Tab/Up/Down] Navigate  [Enter] Login  [Ctrl+C] Exit") + "\n"

	if m.loading {
		loginForm += "\n" + lipgloss.NewStyle().Foreground(greenAccent).Render("Logging in...")
	}

	logoHeight := len(logoLines)
	formHeight := 5
	topPadding := (logoHeight - formHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	paddedLoginForm := strings.Repeat("\n", topPadding) + loginForm
	spacer := lipgloss.NewStyle().Render("   ")

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		coloredLogo,
		spacer,
		paddedLoginForm,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m model) renderReportsPanel() string {
	panelWidth := (m.width - 8) * 40 / 100
	if panelWidth < 40 {
		panelWidth = 40
	}

	maxItems := (m.height - 12) / 2
	if maxItems < 3 {
		maxItems = 3
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(greenAccent).Render("[R] REPORTS (PDF Batches)")

	if len(m.reports) == 0 {
		return panelStyle.Width(panelWidth).Render(title + "\n\nNo reports found")
	}

	startIdx := 0
	endIdx := len(m.reports)
	if len(m.reports) > maxItems {
		startIdx = m.selectedReportIdx - maxItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxItems
		if endIdx > len(m.reports) {
			endIdx = len(m.reports)
			startIdx = endIdx - maxItems
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	var items []string
	for i := startIdx; i < endIdx; i++ {
		r := m.reports[i]
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		line := fmt.Sprintf("Batch[%d]-[%s]-[%s]", i+1, r.Created[:10], shortID)
		if m.activePanel == 0 && i == m.selectedReportIdx {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		if r.Verified {
			line += " [✔]"
		}
		items = append(items, line)
	}

	if len(m.reports) > maxItems {
		scrollPos := (m.selectedReportIdx * 100) / len(m.reports)
		items = append(items, fmt.Sprintf("\n   Scroll: %d%%", scrollPos))
	}

	return panelStyle.Width(panelWidth).Render(title + "\n\n" + strings.Join(items, "\n"))
}

func (m model) renderGeoTagsPanel() string {
	panelWidth := (m.width - 8) * 40 / 100
	if panelWidth < 40 {
		panelWidth = 40
	}

	maxItems := (m.height - 12) / 2
	if maxItems < 3 {
		maxItems = 3
	}

	batchInfo := ""
	if m.currentReport != nil {
		batchInfo = fmt.Sprintf("Report[%d][%s]", m.selectedReportIdx+1, m.currentReport.ID[:8])
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(greenAccent).Render("[G] GEOTAGS (Individual Reports) " + batchInfo)

	if len(m.filteredGeotags) == 0 {
		if m.currentReport == nil {
			return panelStyle.Width(panelWidth).Render(title + "\n\nSelect a report batch to view its geotags")
		}
		return panelStyle.Width(panelWidth).Render(title + "\n\nNo geotags linked to this report batch")
	}

	startIdx := 0
	endIdx := len(m.filteredGeotags)
	if len(m.filteredGeotags) > maxItems {
		startIdx = m.selectedGeotagIdx - maxItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxItems
		if endIdx > len(m.filteredGeotags) {
			endIdx = len(m.filteredGeotags)
			startIdx = endIdx - maxItems
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	var items []string
	for i := startIdx; i < endIdx; i++ {
		g := m.filteredGeotags[i]
		category := g.Category
		if category == "" {
			category = "Unknown"
		}

		categoryPadded := fmt.Sprintf("%-15s", category)
		if len(categoryPadded) > 15 {
			categoryPadded = categoryPadded[:15]
		}

		coordStr := fmt.Sprintf("%8.4f,%8.4f", g.Location.Lat, g.Location.Lon)
		line := fmt.Sprintf("%s - %s", categoryPadded, coordStr)

		if len(line) > panelWidth-6 {
			line = line[:panelWidth-9] + "..."
		}

		if m.activePanel == 1 && i == m.selectedGeotagIdx {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		items = append(items, line)
	}

	if len(m.filteredGeotags) > maxItems {
		scrollPos := (m.selectedGeotagIdx * 100) / len(m.filteredGeotags)
		items = append(items, fmt.Sprintf("\n   Scroll: %d%%", scrollPos))
	}

	return panelStyle.Width(panelWidth).Render(title + "\n\n" + strings.Join(items, "\n"))
}

func (m model) renderDetailsPanel() string {
	panelWidth := m.width - 8
	if panelWidth < 50 {
		panelWidth = 50
	}

	var content string

	if m.currentReport != nil {
		r := m.currentReport
		verified := ""
		if m.verifying && m.verifyingForReportID == r.ID {
			spinner := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
			frame := int(time.Now().UnixMilli()/100) % len(spinner)
			verified = fmt.Sprintf("[%s] Verifying...", spinner[frame])
		} else if r.Verified {
			verified = "[✔] YES"
		} else {
			verified = "[X] NO"
		}

		pdfStatus := "[?] NO PDF"
		if r.Document != "" {
			if m.downloadingPDF && m.downloadingForReportID == r.ID {
				spinner := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
				frame := int(time.Now().UnixMilli()/100) % len(spinner)
				pdfStatus = fmt.Sprintf("[%s] Downloading %s...", spinner[frame], r.Document)
			} else {
				pdfStatus = "[✔] " + r.Document
				if len(pdfStatus) > panelWidth-10 {
					pdfStatus = pdfStatus[:panelWidth-13] + "..."
				}
			}
		}

		content = fmt.Sprintf(
			"Selected: Report Batch #%d\n\n"+
				"Created: %s\n"+
				"Verified: %s\n"+
				"Emailed: %v\n\n"+
				"PDF: %s\n",
			m.selectedReportIdx+1, r.Created[:10], verified, r.Emailed,
			pdfStatus,
		)
	} else if m.currentGeotag != nil {
		g := m.currentGeotag
		emailed := "[X] NO"
		if g.Emailed {
			emailed = "[✔] YES"
		}

		category := g.Category
		if category == "" {
			category = "Uncategorized"
		}

		photoStatus := "[X] NO PHOTO"
		if g.Image != "" {
			photoStatus = "[✔] " + g.Image
			if len(photoStatus) > panelWidth-10 {
				photoStatus = photoStatus[:panelWidth-13] + "..."
			}
		}

		content = fmt.Sprintf(
			"Selected: Report #%s\n\n"+
				"Category: %s\n"+
				"Coordinates: %.6f, %.6f\n"+
				"Created: %s\n"+
				"Governorate: %s\n"+
				"Emailed: %s\n"+
				"Photo: %s\n",
			g.ID[:8], category, g.Location.Lat, g.Location.Lon,
			g.Created[:10], g.Governorate, emailed,
			photoStatus,
		)
	} else {
		content = "Select a report from the left panels to view details"
	}

	buttons := "\n\n"
	if m.currentReport != nil {
		buttons += buttonStyle.Render("[V] Verify/Unverify")
		if m.currentReport.Document != "" {
			buttons += buttonStyle.Render("[P] View PDF")
		}
	}
	if m.activePanel == 1 && m.currentGeotag != nil {
		buttons += buttonStyle.Render("[O] Open in Map")
	}

	snackbarContent := ""
	if m.snackbar != "" {
		snackColor := greenAccent
		if m.snackbarType == "error" {
			snackColor = red
		}
		separator := strings.Repeat("─", panelWidth-4)
		snackbarContent = "\n\n" + separator + "\n" +
			snackbarStyle.Foreground(snackColor).Width(panelWidth-4).Render(m.snackbar)
	}

	return detailStyle.Width(panelWidth).Render(content + buttons + snackbarContent)
}

func (m model) renderMainScreen() string {
	titleText := fmt.Sprintf("Bayanati - %s verify dashboard", strings.Title(m.governorate))
	titleWidth := m.width - 20
	if titleWidth < len(titleText)+10 {
		titleWidth = len(titleText) + 10
	}
	header := titleStyle.Width(titleWidth).Render(titleText)
	header += "  " + lipgloss.NewStyle().Foreground(mutedColor).Render("[Ctrl+C] Exit")

	leftPanel := m.renderReportsPanel()
	rightPanel := m.renderGeoTagsPanel()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	details := m.renderDetailsPanel()

	footer := lipgloss.NewStyle().Foreground(mutedColor).Render(
		"[Tab] Switch Panels  [Up/Down] Navigate  [V] Verify Report  [P] View PDF  [O] Open Map (on Geotags)",
	)

	body := lipgloss.JoinVertical(lipgloss.Left, topRow, "\n", details)

	return lipgloss.JoinVertical(lipgloss.Left, header, "\n", body, "\n", footer)
}

func (m model) View() string {
	if !m.loggedIn {
		return m.renderLoginScreen()
	}
	return m.renderMainScreen()
}

func main() {
	model := initialModel()
	defer model.cleanupCache()
	p := tea.NewProgram(&model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
