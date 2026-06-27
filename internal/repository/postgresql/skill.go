package postgresql

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"psa/internal/domain"
	generated "psa/internal/repository/postgresql/generated"
)

func (s *Storage) SaveFormalSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID,
	skills map[string]int) error {

	return insertFormalSkills(ctx, s.Queries, sessionID, professionID, skills)
}

func (s *Storage) SaveExtractedSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID,
	skills map[string]int) error {

	return insertExtractedSkills(ctx, s.Queries, sessionID, professionID, skills)
}

func insertFormalSkills(ctx context.Context, q *generated.Queries, sessionID uuid.UUID,
	professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.skill.insertFormalSkills"

	if len(skills) == 0 {
		return nil
	}

	params := make([]generated.InsertFormalSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, generated.InsertFormalSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	if _, err := q.InsertFormalSkills(ctx, params); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func insertExtractedSkills(ctx context.Context, q *generated.Queries, sessionID uuid.UUID,
	professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.skill.insertExtractedSkills"

	if len(skills) == 0 {
		return nil
	}

	params := make([]generated.InsertExtractedSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, generated.InsertExtractedSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	if _, err := q.InsertExtractedSkills(ctx, params); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]domain.Skill, error) {
	const op = "repository.postgresql.skill.GetFormalSkillsByProfessionAndDate"

	rows, err := s.Queries.GetFormalSkillsByProfessionAndDate(ctx, generated.GetFormalSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]domain.Skill, len(rows))
	for i, row := range rows {
		skills[i] = domain.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}

func (s *Storage) GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]domain.Skill, error) {
	const op = "repository.postgresql.skill.GetExtractedSkillsByProfessionAndDate"

	rows, err := s.Queries.GetExtractedSkillsByProfessionAndDate(ctx, generated.GetExtractedSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]domain.Skill, len(rows))
	for i, row := range rows {
		skills[i] = domain.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}
