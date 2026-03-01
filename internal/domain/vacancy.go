package domain

type VacancyData struct {
	Skills      []string `json:"skills"`
	Description string   `json:"description"`
	TotalFound  int      `json:"total_found"`
}

type SkillData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
