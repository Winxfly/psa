package postgresql

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"psa/internal/entity"
	postgresql "psa/internal/repository/postgresql/generated"
)

func (s *Storage) SaveFormalSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.skill.SaveFormalSkills"

	params := make([]postgresql.InsertFormalSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, postgresql.InsertFormalSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	_, err := s.Queries.InsertFormalSkills(ctx, params)

	return err
}

func (s *Storage) SaveExtractedSkills(ctx context.Context, sessionID uuid.UUID, professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.skill.SaveExtractedSkills"

	params := make([]postgresql.InsertExtractedSkillsParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, postgresql.InsertExtractedSkillsParams{
			ProfessionID: professionID,
			Skill:        skill,
			Count:        int32(count),
			ScrapedAtID:  sessionID,
		})
	}

	_, err := s.Queries.InsertExtractedSkills(ctx, params)

	return err
}

func (s *Storage) GetFormalSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error) {
	const op = "repository.postgresql.skill.GetFormalSkillsByProfessionAndDate"

	rows, err := s.Queries.GetFormalSkillsByProfessionAndDate(ctx, postgresql.GetFormalSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]entity.Skill, len(rows))
	for i, row := range rows {
		skills[i] = entity.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}

func (s *Storage) GetExtractedSkillsByProfessionAndDate(ctx context.Context, professionID uuid.UUID, scrapedAtID uuid.UUID) ([]entity.Skill, error) {
	const op = "repository.postgresql.skill.GetExtractedSkillsByProfessionAndDate"

	rows, err := s.Queries.GetExtractedSkillsByProfessionAndDate(ctx, postgresql.GetExtractedSkillsByProfessionAndDateParams{
		ProfessionID: professionID,
		ScrapedAtID:  scrapedAtID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	skills := make([]entity.Skill, len(rows))
	for i, row := range rows {
		skills[i] = entity.Skill{
			Skill: row.Skill,
			Count: row.Count,
		}
	}

	return skills, nil
}
