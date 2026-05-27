package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func renderLoginScreen(m Model) string {
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
	formWidth := m.Width - logoWidth - 10
	if formWidth < 40 {
		formWidth = 40
	}

	loginForm := titleStyle.Width(formWidth).Render("Bayanati Municipality Portal") + "\n\n"

	if m.LoginError != "" {
		loginForm += errorStyle.Render("Error: "+m.LoginError) + "\n\n"
	}

	emailField := "Email: " + m.Email
	if m.FocusedField == 0 {
		emailField = selectedStyle.Render("> " + emailField + " <")
	} else {
		emailField = "  " + emailField
	}

	passwordDisplay := strings.Repeat("*", len(m.Password))
	passwordField := "Password: " + passwordDisplay
	if m.FocusedField == 1 {
		passwordField = selectedStyle.Render("> " + passwordField + " <")
	} else {
		passwordField = "  " + passwordField
	}

	loginForm += emailField + "\n" + passwordField + "\n\n"
	loginForm += lipgloss.NewStyle().Foreground(mutedColor).Render("[Tab/Up/Down] Navigate  [Enter] Login  [Ctrl+C] Exit") + "\n"

	if m.Loading {
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

func renderReportsPanel(m Model) string {
	panelWidth := (m.Width - 8) * 40 / 100
	if panelWidth < 40 {
		panelWidth = 40
	}

	maxItems := (m.Height - 12) / 2
	if maxItems < 3 {
		maxItems = 3
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(greenAccent).Render("[R] REPORTS (PDF Batches)")

	if len(m.Reports) == 0 {
		return panelStyle.Width(panelWidth).Render(title + "\n\nNo reports found")
	}

	startIdx := 0
	endIdx := len(m.Reports)
	if len(m.Reports) > maxItems {
		startIdx = m.SelectedReportIdx - maxItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxItems
		if endIdx > len(m.Reports) {
			endIdx = len(m.Reports)
			startIdx = endIdx - maxItems
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	var items []string
	for i := startIdx; i < endIdx; i++ {
		r := m.Reports[i]
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		line := fmt.Sprintf("Batch[%d]-[%s]-[%s]", i+1, r.Created[:10], shortID)
		if m.ActivePanel == 0 && i == m.SelectedReportIdx {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		if r.Verified {
			line += " [✔]"
		}
		items = append(items, line)
	}

	if len(m.Reports) > maxItems {
		scrollPos := (m.SelectedReportIdx * 100) / len(m.Reports)
		items = append(items, fmt.Sprintf("\n   Scroll: %d%%", scrollPos))
	}

	return panelStyle.Width(panelWidth).Render(title + "\n\n" + strings.Join(items, "\n"))
}

func renderGeoTagsPanel(m Model) string {
	panelWidth := (m.Width - 8) * 40 / 100
	if panelWidth < 40 {
		panelWidth = 40
	}

	maxItems := (m.Height - 12) / 2
	if maxItems < 3 {
		maxItems = 3
	}

	batchInfo := ""
	if m.CurrentReport != nil {
		batchInfo = fmt.Sprintf("Report[%d][%s]", m.SelectedReportIdx+1, m.CurrentReport.ID[:8])
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(greenAccent).Render("[G] GEOTAGS (Individual Reports) " + batchInfo)

	if len(m.FilteredGeotags) == 0 {
		if m.CurrentReport == nil {
			return panelStyle.Width(panelWidth).Render(title + "\n\nSelect a report batch to view its geotags")
		}
		return panelStyle.Width(panelWidth).Render(title + "\n\nNo geotags linked to this report batch")
	}

	startIdx := 0
	endIdx := len(m.FilteredGeotags)
	if len(m.FilteredGeotags) > maxItems {
		startIdx = m.SelectedGeotagIdx - maxItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxItems
		if endIdx > len(m.FilteredGeotags) {
			endIdx = len(m.FilteredGeotags)
			startIdx = endIdx - maxItems
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	var items []string
	for i := startIdx; i < endIdx; i++ {
		g := m.FilteredGeotags[i]
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

		if m.ActivePanel == 1 && i == m.SelectedGeotagIdx {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		items = append(items, line)
	}

	if len(m.FilteredGeotags) > maxItems {
		scrollPos := (m.SelectedGeotagIdx * 100) / len(m.FilteredGeotags)
		items = append(items, fmt.Sprintf("\n   Scroll: %d%%", scrollPos))
	}

	return panelStyle.Width(panelWidth).Render(title + "\n\n" + strings.Join(items, "\n"))
}

func renderDetailsPanel(m Model) string {
	panelWidth := m.Width - 8
	if panelWidth < 50 {
		panelWidth = 50
	}

	var content string

	if m.CurrentReport != nil {
		r := m.CurrentReport
		verified := ""
		if m.Verifying && m.VerifyingForReportID == r.ID {
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
			if m.DownloadingPDF && m.DownloadingForReportID == r.ID {
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
			m.SelectedReportIdx+1, r.Created[:10], verified, r.Emailed,
			pdfStatus,
		)
	} else if m.CurrentGeotag != nil {
		g := m.CurrentGeotag
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
	if m.CurrentReport != nil {
		buttons += buttonStyle.Render("[V] Verify/Unverify")
		if m.CurrentReport.Document != "" {
			buttons += buttonStyle.Render("[P] View PDF")
		}
	}
	if m.ActivePanel == 1 && m.CurrentGeotag != nil {
		buttons += buttonStyle.Render("[O] Open in Map")
	}

	snackbarContent := ""
	if m.Snackbar != "" {
		snackColor := greenAccent
		if m.SnackbarType == "error" {
			snackColor = red
		}
		separator := strings.Repeat("─", panelWidth-4)
		snackbarContent = "\n\n" + separator + "\n" +
			snackbarStyle.Foreground(snackColor).Width(panelWidth-4).Render(m.Snackbar)
	}

	return detailStyle.Width(panelWidth).Render(content + buttons + snackbarContent)
}

func renderMainScreen(m Model) string {
	titleText := fmt.Sprintf("Bayanati - %s verify dashboard", strings.Title(m.Governorate))
	titleWidth := m.Width - 20
	if titleWidth < len(titleText)+10 {
		titleWidth = len(titleText) + 10
	}
	header := titleStyle.Width(titleWidth).Render(titleText)
	header += "  " + lipgloss.NewStyle().Foreground(mutedColor).Render("[Ctrl+C] Exit")

	leftPanel := renderReportsPanel(m)
	rightPanel := renderGeoTagsPanel(m)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	details := renderDetailsPanel(m)

	footer := lipgloss.NewStyle().Foreground(mutedColor).Render(
		"[Tab] Switch Panels  [Up/Down] Navigate  [V] Verify Report  [P] View PDF  [O] Open Map (on Geotags)",
	)

	body := lipgloss.JoinVertical(lipgloss.Left, topRow, "\n", details)

	return lipgloss.JoinVertical(lipgloss.Left, header, "\n", body, "\n", footer)
}

func view(m Model) string {
	if !m.LoggedIn {
		return renderLoginScreen(m)
	}
	return renderMainScreen(m)
}
