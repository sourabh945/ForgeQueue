package types

type Job struct {
	JobId     string `json:"JobId"`
	ImagePath string `json:"ImagePath"`
	Type      string `json:"Type"`
}

type JobResponse struct {
	JobId     string `json:"JobId"`
	ImagePath string `json:"ImagePath"`
	Status    string `json:"Status"`
}
