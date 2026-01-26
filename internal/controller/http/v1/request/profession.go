package request

type CreateProfessionRequest struct {
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
}

type UpdateProfessionRequest struct {
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
	IsActive     bool   `json:"is_active"`
}
