package interfaces

import (
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type TaskDispatch struct {
	TaskID      string
	Type        common.TaskType
	Parameters  map[string]interface{}
	Timeout     int
	Priority    int
	MissionID   string
}

type TaskResult struct {
	TaskID   string
	Status   common.TaskStatus
	Output   []byte
	Evidence []mission.EvidenceRef
	Duration int
	Error    *mission.TaskError
}
