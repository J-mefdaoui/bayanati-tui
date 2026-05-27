package main

import "time"

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

type Model struct {
	// Auth
	Email        string
	Password     string
	Token        string
	Governorate  string
	LoggedIn     bool
	LoginError   string
	FocusedField int

	// Data
	Reports           []Report
	Geotags           []GeoTags
	FilteredGeotags   []GeoTags
	SelectedReportIdx int
	SelectedGeotagIdx int
	ActivePanel       int

	// Selection
	CurrentReport *Report
	CurrentGeotag *GeoTags

	// UI state
	Loading           bool
	Snackbar          string
	SnackbarType      string
	SnackbarExpiry    time.Time
	Verifying         bool
	VerifyingForReportID string

	// Window dimensions
	Width  int
	Height int

	// cache feature
	DownloadedFiles        map[string]string
	DownloadingPDF         bool
	DownloadingForReportID string

	//TODO: search feature
	// Search & Pagination
  SearchQuery     string
  FilteredReports []Report
  CurrentPage     int
  ItemsPerPage    int
  TotalPages      int
  SearchFocused   bool
}

// Message types
type LoginMsg struct {
	Success     bool
	Err         error
	Token       string
	Governorate string
}

type FetchReportsMsg struct {
	Reports []Report
	Err     error
}

type FetchGeoTagsMsg struct {
	Geotags []GeoTags
	Err     error
}

type UpdateMsg struct {
	Success  bool
	ID       string
	Verified bool
	Err      error
}

type PDFMsg struct {
	Success  bool
	ReportID string
	Filepath string
	Err      error
	Cached   bool
}

type SnackbarMsg struct {
	Message string
	MsgType string
}

type TickMsg struct{}

func InitialModel() Model {
	return Model{
		Email:             "",
		Password:          "",
		LoggedIn:          false,
		FocusedField:      0,
		ActivePanel:       0,
		SelectedReportIdx: 0,
		SelectedGeotagIdx: 0,
		Reports:           []Report{},
		Geotags:           []GeoTags{},
		FilteredGeotags:   []GeoTags{},
		Width:             80,
		Height:            24,
		DownloadedFiles:   make(map[string]string),
		CurrentPage:			 1,
		ItemsPerPage:			 10,
		FilteredReports: []Report{},
		SearchFocused: false,
	}
}
