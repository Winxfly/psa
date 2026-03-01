package hh

type metadata struct {
	Found int `json:"found"`
	Pages int `json:"pages"`
}

type vacancyIDResponse struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
}

type vacancyResponse struct {
	Description string `json:"description"`
	KeySkills   []struct {
		Name string `json:"name"`
	} `json:"key_skills"`
}

type professionData struct {
	Vacancies  []vacancyResponse
	TotalFound int
}
