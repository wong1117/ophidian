package dto

type GenerateReportRequest struct {
	MissionID string `json:"mission_id"`
	Format    string `json:"format"`
}

type ReportResponse struct {
	Data     []byte `json:"data"`
	Format   string `json:"format"`
	Filename string `json:"filename"`
}
