package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type MissionRepository struct {
	deps RepoDeps
}

func NewMissionRepository(pool *pgxpool.Pool) *MissionRepository {
	return &MissionRepository{deps: repoDepsFromPool(pool)}
}

func NewMissionRepositoryWithDeps(deps RepoDeps) *MissionRepository {
	return &MissionRepository{deps: deps}
}

func (r *MissionRepository) Save(ctx context.Context, m *mission.Mission) error {
	targetJSON := marshalJSON(m.Target)
	roeJSON := marshalJSON(m.RoE)
	phasesJSON := marshalJSON(m.Phases)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO missions (id, name, status, target, roe, phases, created_at, updated_at, started_by, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)`,
		m.ID, m.Name, m.Status, targetJSON, roeJSON, phasesJSON,
		m.CreatedAt, m.UpdatedAt, m.StartedBy,
	)
	return wrapSaveError(err, "mission")
}

func (r *MissionRepository) FindByID(ctx context.Context, id string) (*mission.Mission, error) {
	var m mission.Mission
	var targetJSON, roeJSON, phasesJSON []byte

	err := r.deps.QueryRow(ctx,
		`SELECT id, name, status, target, roe, phases, created_at, updated_at, started_by
		 FROM missions WHERE id = $1`, id,
	).Scan(&m.ID, &m.Name, &m.Status, &targetJSON, &roeJSON, &phasesJSON,
		&m.CreatedAt, &m.UpdatedAt, &m.StartedBy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: mission %s not found", common.ErrMissionNotFound, id)
		}
		return nil, fmt.Errorf("find mission by id: %w", err)
	}

	if err := unmarshalJSON(targetJSON, &m.Target); err != nil {
		return nil, fmt.Errorf("find mission: unmarshal target: %w", err)
	}
	if err := unmarshalJSON(roeJSON, &m.RoE); err != nil {
		return nil, fmt.Errorf("find mission: unmarshal roe: %w", err)
	}
	if err := unmarshalJSON(phasesJSON, &m.Phases); err != nil {
		return nil, fmt.Errorf("find mission: unmarshal phases: %w", err)
	}

	return &m, nil
}

func (r *MissionRepository) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	query := `SELECT id, name, status, target, roe, phases, created_at, updated_at, started_by
		 FROM missions WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	rows, err := r.deps.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find all missions: %w", err)
	}
	defer rows.Close()

	var missions []*mission.Mission
	for rows.Next() {
		var m mission.Mission
		var targetJSON, roeJSON, phasesJSON []byte
		if err := rows.Scan(&m.ID, &m.Name, &m.Status, &targetJSON, &roeJSON, &phasesJSON,
			&m.CreatedAt, &m.UpdatedAt, &m.StartedBy); err != nil {
			return nil, fmt.Errorf("find all missions: scan: %w", err)
		}
		if err := unmarshalJSON(targetJSON, &m.Target); err != nil {
			return nil, fmt.Errorf("find all missions: unmarshal target: %w", err)
		}
		if err := unmarshalJSON(roeJSON, &m.RoE); err != nil {
			return nil, fmt.Errorf("find all missions: unmarshal roe: %w", err)
		}
		if err := unmarshalJSON(phasesJSON, &m.Phases); err != nil {
			return nil, fmt.Errorf("find all missions: unmarshal phases: %w", err)
		}
		missions = append(missions, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all missions: rows: %w", err)
	}

	return missions, nil
}

func (r *MissionRepository) Update(ctx context.Context, m *mission.Mission) error {
	targetJSON := marshalJSON(m.Target)
	roeJSON := marshalJSON(m.RoE)
	phasesJSON := marshalJSON(m.Phases)

	tag, err := r.deps.Exec(ctx,
		`UPDATE missions SET name = $1, status = $2, target = $3, roe = $4, phases = $5,
		 updated_at = $6, started_by = $7, version = version + 1
		 WHERE id = $8`,
		m.Name, m.Status, targetJSON, roeJSON, phasesJSON,
		m.UpdatedAt, m.StartedBy, m.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "mission")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: mission %s not found for update", common.ErrMissionNotFound, m.ID)
	}

	return nil
}

func (r *MissionRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.deps.Exec(ctx, `DELETE FROM missions WHERE id = $1`, id)
	if err != nil {
		return wrapDeleteError(err, "mission")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: mission %s not found", common.ErrMissionNotFound, id)
	}
	return nil
}

func (r *MissionRepository) SaveTask(ctx context.Context, task *mission.Task) error {
	paramsJSON := marshalJSON(task.Parameters)
	resultJSON := marshalJSON(task.Result)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO mission_tasks (id, mission_id, type, status, priority, parameters, timeout,
		 retry_count, max_retries, depends_on, assigned_to, created_at, started_at, completed_at, result)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		task.ID, task.MissionID, task.Type, task.Status, task.Priority, paramsJSON, task.Timeout,
		task.RetryCount, task.MaxRetries, marshalJSON(task.DependsOn), task.AssignedTo,
		task.CreatedAt, task.StartedAt, task.CompletedAt, resultJSON,
	)
	return wrapSaveError(err, "task")
}

func (r *MissionRepository) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) {
	var t mission.Task
	var paramsJSON, dependsOnJSON, resultJSON []byte

	err := r.deps.QueryRow(ctx,
		`SELECT id, mission_id, type, status, priority, parameters, timeout,
		 retry_count, max_retries, depends_on, assigned_to, created_at, started_at, completed_at, result
		 FROM mission_tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.MissionID, &t.Type, &t.Status, &t.Priority, &paramsJSON,
		&t.Timeout, &t.RetryCount, &t.MaxRetries, &dependsOnJSON,
		&t.AssignedTo, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &resultJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: task %s not found", common.ErrTaskNotFound, id)
		}
		return nil, fmt.Errorf("find task by id: %w", err)
	}

	if err := unmarshalJSON(paramsJSON, &t.Parameters); err != nil {
		return nil, fmt.Errorf("find task: unmarshal params: %w", err)
	}
	json.Unmarshal(dependsOnJSON, &t.DependsOn)
	if resultJSON != nil && string(resultJSON) != "null" {
		var result mission.TaskResult
		if err := unmarshalJSON(resultJSON, &result); err != nil {
			return nil, fmt.Errorf("find task: unmarshal result: %w", err)
		}
		t.Result = &result
	}

	return &t, nil
}

func (r *MissionRepository) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, mission_id, type, status, priority, parameters, timeout,
		 retry_count, max_retries, depends_on, assigned_to, created_at, started_at, completed_at, result
		 FROM mission_tasks WHERE mission_id = $1 ORDER BY priority ASC`, missionID,
	)
	if err != nil {
		return nil, fmt.Errorf("find tasks by mission: %w", err)
	}
	defer rows.Close()

	var tasks []*mission.Task
	for rows.Next() {
		var t mission.Task
		var paramsJSON, dependsOnJSON, resultJSON []byte
		if err := rows.Scan(&t.ID, &t.MissionID, &t.Type, &t.Status, &t.Priority,
			&paramsJSON, &t.Timeout, &t.RetryCount, &t.MaxRetries, &dependsOnJSON,
			&t.AssignedTo, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &resultJSON); err != nil {
			return nil, fmt.Errorf("find tasks by mission: scan: %w", err)
		}
		json.Unmarshal(paramsJSON, &t.Parameters)
		json.Unmarshal(dependsOnJSON, &t.DependsOn)
		if resultJSON != nil && string(resultJSON) != "null" {
			var result mission.TaskResult
			if err := unmarshalJSON(resultJSON, &result); err != nil {
				return nil, fmt.Errorf("find tasks by mission: unmarshal result: %w", err)
			}
			t.Result = &result
		}
		tasks = append(tasks, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find tasks by mission: rows: %w", err)
	}

	return tasks, nil
}

func (r *MissionRepository) UpdateTask(ctx context.Context, task *mission.Task) error {
	paramsJSON := marshalJSON(task.Parameters)
	resultJSON := marshalJSON(task.Result)

	tag, err := r.deps.Exec(ctx,
		`UPDATE mission_tasks SET status = $1, parameters = $2, retry_count = $3,
		 assigned_to = $4, started_at = $5, completed_at = $6, result = $7
		 WHERE id = $8`,
		task.Status, paramsJSON, task.RetryCount, task.AssignedTo,
		task.StartedAt, task.CompletedAt, resultJSON, task.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "task")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: task %s not found for update", common.ErrTaskNotFound, task.ID)
	}
	return nil
}
