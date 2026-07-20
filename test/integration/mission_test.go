package integration

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

func TestMissionLifecycle(t *testing.T) {
	m := &mission.Mission{
		ID:   common.NewID(),
		Name: "test-mission",
		Status: mission.MissionDraft,
		CreatedAt: common.Now(),
	}

	agg := mission.NewMissionAggregate(m)
	err := agg.Start()
	assert.NoError(t, err)
	assert.Equal(t, mission.MissionActive, m.Status)
}
