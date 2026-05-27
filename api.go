package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)


func getTempDir() string {
	if runtime.GOOS == "windows" {
		return os.TempDir()
	}
	return "/tmp"
}

func (m *Model) login() tea.Cmd {
	m.Loading = true
	m.LoginError = ""
	return func() tea.Msg {
		data := strings.NewReader(fmt.Sprintf(`{"identity":"%s","password":"%s"}`, m.Email, m.Password))
		req, _ := http.NewRequest("POST", PB_URL+"/api/collections/users/auth-with-password", data)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return LoginMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return LoginMsg{Err: fmt.Errorf("login failed: %s", string(body))}
		}

		var result struct {
			Token string `json:"token"`
			Record struct {
				Governorate string `json:"governorate"`
			} `json:"record"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		return LoginMsg{Success: true, Token: result.Token, Governorate: result.Record.Governorate}
	}
}

func (m *Model) fetchReports() tea.Cmd {
	return func() tea.Msg {
		req, _ := http.NewRequest("GET", PB_URL+"/api/collections/Reports/records?sort=-created", nil)
		req.Header.Set("Authorization", m.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return FetchReportsMsg{Err: err}
		}
		defer resp.Body.Close()

		var result struct {
			Items []Report `json:"items"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		return FetchReportsMsg{Reports: result.Items}
	}
}

func (m *Model) fetchGeoTags() tea.Cmd {
	return func() tea.Msg {
		var allGeotags []GeoTags
		page := 1

		for {
			url := fmt.Sprintf("%s/api/collections/GeoTags/records?sort=-created&perPage=100&page=%d", PB_URL, page)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", m.Token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return FetchGeoTagsMsg{Err: err}
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

		return FetchGeoTagsMsg{Geotags: allGeotags}
	}
}

func (m *Model) updateReport(reportID string, verified bool) tea.Cmd {
	return func() tea.Msg {
		data := strings.NewReader(fmt.Sprintf(`{"verified":%v}`, verified))
		req, _ := http.NewRequest("PATCH", PB_URL+"/api/collections/Reports/records/"+reportID, data)
		req.Header.Set("Authorization", m.Token)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return UpdateMsg{Err: err}
		}
		defer resp.Body.Close()

		return UpdateMsg{Success: true, ID: reportID, Verified: verified}
	}
}

func (m *Model) downloadPDF(reportID, filename string) tea.Cmd {
	// Check if we already downloaded this report
	if existingPath, exists := m.DownloadedFiles[reportID]; exists {
		if _, err := os.Stat(existingPath); err == nil {
			return func() tea.Msg {
				return PDFMsg{Success: true, ReportID: reportID, Filepath: existingPath, Cached: true}
			}
		} else {
			delete(m.DownloadedFiles, reportID)
		}
	}

	return func() tea.Msg {
		fileURL := fmt.Sprintf("%s/api/files/Reports/%s/%s", PB_URL, reportID, filename)

		req, err := http.NewRequest("GET", fileURL, nil)
		if err != nil {
			return PDFMsg{Err: fmt.Errorf("failed to create request: %v", err), ReportID: reportID}
		}
		req.Header.Set("Authorization", m.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return PDFMsg{Err: fmt.Errorf("failed to download: %v", err), ReportID: reportID}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return PDFMsg{Err: fmt.Errorf("PDF download failed (status %d): %s", resp.StatusCode, string(body)), ReportID: reportID}
		}

		timestamp := time.Now().Unix()
		safeFilename := strings.ReplaceAll(filename, "/", "_")
		safeFilename = strings.ReplaceAll(safeFilename, "\\", "_")
		outFile := fmt.Sprintf("%s/bayanati_%s_%d_%s", getTempDir(), reportID[:8], timestamp, safeFilename)

		out, err := os.Create(outFile)
		if err != nil {
			return PDFMsg{Err: fmt.Errorf("failed to create file: %v", err), ReportID: reportID}
		}
		defer out.Close()

		written, err := io.Copy(out, resp.Body)
		if err != nil {
			return PDFMsg{Err: fmt.Errorf("failed to save PDF: %v", err), ReportID: reportID}
		}

		if written == 0 {
			return PDFMsg{Err: fmt.Errorf("downloaded PDF is empty"), ReportID: reportID}
		}

		m.DownloadedFiles[reportID] = outFile

		return PDFMsg{Success: true, ReportID: reportID, Filepath: outFile, Cached: false}
	}
}

func openPDF(filepath string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			winPath := filepath
			cmd = exec.Command("cmd", "/c", "start", "", winPath)
			if err := cmd.Run(); err != nil {
				cmd = exec.Command("explorer", winPath)
				if err := cmd.Run(); err != nil {
					return SnackbarMsg{Message: fmt.Sprintf("Failed to open PDF: %v", err), MsgType: "error"}
				}
			}
			return SnackbarMsg{Message: "PDF opened with default viewer", MsgType: "success"}
		case "darwin":
			cmd = exec.Command("open", filepath)
		default:
			cmd = exec.Command("xdg-open", filepath)
		}

		if err := cmd.Run(); err != nil {
			return SnackbarMsg{Message: fmt.Sprintf("Failed to open PDF: %v", err), MsgType: "error"}
		}
		return SnackbarMsg{Message: "PDF opened with default viewer", MsgType: "success"}
	}
}

func openCoordinates(lat, lon float64) tea.Cmd {
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
			return SnackbarMsg{Message: fmt.Sprintf("Failed to open map: %v", err), MsgType: "error"}
		}
		return SnackbarMsg{Message: fmt.Sprintf("Opening coordinates: %.6f, %.6f", lat, lon), MsgType: "info"}
	}
}
