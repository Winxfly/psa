package entity

type VacancyData struct {
	Skills      []string `json:"skills"`
	Description string   `json:"description"`
}

type SkillData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
