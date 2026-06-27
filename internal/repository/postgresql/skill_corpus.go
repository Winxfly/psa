package postgresql

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	generated "psa/internal/repository/postgresql/generated"
)

func (s *Storage) GetActiveSkillCorpus(ctx context.Context) (map[uuid.UUID]map[string]int, error) {
	const op = "repository.postgresql.skill_corpus.GetActiveSkillCorpus"

	rows, err := s.Queries.GetActiveSkillCorpus(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	corpus := make(map[uuid.UUID]map[string]int)
	for _, row := range rows {
		if _, ok := corpus[row.ProfessionID]; !ok {
			corpus[row.ProfessionID] = make(map[string]int)
		}

		corpus[row.ProfessionID][row.Skill] = int(row.MentionCount)
	}

	return corpus, nil
}

func replaceSkillCorpusByProfession(ctx context.Context, q *generated.Queries, professionID uuid.UUID, skills map[string]int) error {
	const op = "repository.postgresql.skill_corpus.replaceSkillCorpusByProfession"

	if err := q.DeleteSkillCorpusByProfession(ctx, professionID); err != nil {
		return fmt.Errorf("%s: delete corpus: %w", op, err)
	}

	if len(skills) == 0 {
		return nil
	}

	params := make([]generated.InsertSkillCorpusParams, 0, len(skills))
	for skill, count := range skills {
		params = append(params, generated.InsertSkillCorpusParams{
			ProfessionID: professionID,
			Skill:        skill,
			MentionCount: int32(count),
		})
	}

	if _, err := q.InsertSkillCorpus(ctx, params); err != nil {
		return fmt.Errorf("%s: insert corpus: %w", op, err)
	}

	return nil
}
