package response

type ProfessionResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
}

type ProfessionAdminResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VacancyQuery string `json:"vacancy_query"`
	IsActive     bool   `json:"is_active"`
}

type ProfessionDetailResponse struct {
	ProfessionID    string          `json:"profession_id"`
	ProfessionName  string          `json:"profession_name"`
	ScrapedAt       string          `json:"scraped_at"`
	VacancyCount    int32           `json:"vacancy_count"`
	FormalSkills    []SkillResponse `json:"formal_skills"`
	ExtractedSkills []SkillResponse `json:"extracted_skills"`
	Trend           []TrendPoint    `json:"trend,omitempty"`
}

type TrendPoint struct {
	Date         string `json:"date"`
	VacancyCount int32  `json:"vacancy_count"`
}

type SkillResponse struct {
	Skill string `json:"skill"`
	Count int32  `json:"count"`
}

type ProfessionTrendResponse struct {
	ProfessionID   string       `json:"profession_id"`
	ProfessionName string       `json:"profession_name"`
	Data           []TrendPoint `json:"data"`
}
