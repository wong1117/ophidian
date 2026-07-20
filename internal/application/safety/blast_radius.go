package safety

import (
	"context"
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type BlastRadiusPreview struct {
	PlanID       string
	MissionID    string
	AffectedHosts []string
	AffectedSubnets []string
	ExcludedHosts   []string
	TargetCount     int
	RiskLevel       common.RiskLevel
	RoESummary      mission.RoEConstraints
	EstimatedImpact string
	RequiresApproval bool
}

type DryRunMode struct{}

func (d *DryRunMode) Simulate(ctx context.Context, plan *attackplan.AttackPlan, mission *mission.Mission) (*BlastRadiusPreview, error) {
	preview := &BlastRadiusPreview{
		PlanID:       plan.ID.String(),
		MissionID:    mission.ID.String(),
		RiskLevel:    common.RiskMedium,
		RequiresApproval: true,
		RoESummary:   mission.RoE,
	}

	affectedMap := make(map[string]bool)
	for _, node := range plan.Graph.Nodes {
		if node.TargetID != "" && !affectedMap[node.TargetID] {
			affectedMap[node.TargetID] = true
			preview.AffectedHosts = append(preview.AffectedHosts, node.TargetID)
		}
	}
	preview.TargetCount = len(preview.AffectedHosts)

	for _, target := range mission.Target.IPs {
		excluded := false
		for _, excl := range mission.RoE.ExcludedNets {
			if target == excl {
				excluded = true
				preview.ExcludedHosts = append(preview.ExcludedHosts, target)
				break
			}
		}
		if !excluded {
			preview.AffectedHosts = append(preview.AffectedHosts, target)
		}
	}

	for _, cidr := range mission.Target.CIDRs {
		preview.AffectedSubnets = append(preview.AffectedSubnets, cidr)
	}

	preview.EstimatedImpact = d.estimateImpact(preview)
	preview.RiskLevel = d.calculateRisk(plan)

	return preview, nil
}

func (d *DryRunMode) estimateImpact(preview *BlastRadiusPreview) string {
	hostCount := len(preview.AffectedHosts)
	subnetCount := len(preview.AffectedSubnets)

	if hostCount > 50 || subnetCount > 5 {
		return fmt.Sprintf("CRITICAL: Will affect %d hosts across %d subnets", hostCount, subnetCount)
	} else if hostCount > 10 {
		return fmt.Sprintf("HIGH: Will affect %d hosts across %d subnets", hostCount, subnetCount)
	} else if hostCount > 0 {
		return fmt.Sprintf("MEDIUM: Will affect %d hosts", hostCount)
	}
	return "LOW: Minimal blast radius"
}

func (d *DryRunMode) calculateRisk(plan *attackplan.AttackPlan) common.RiskLevel {
	maxRisk := common.RiskLow
	for _, path := range plan.RankedPaths {
		if path.RiskLevel > maxRisk {
			maxRisk = path.RiskLevel
		}
	}
	return maxRisk
}

func (d *DryRunMode) Summary(preview *BlastRadiusPreview) string {
	summary := fmt.Sprintf("=== BLAST RADIUS PREVIEW ===\n")
	summary += fmt.Sprintf("Plan: %s\n", preview.PlanID)
	summary += fmt.Sprintf("Risk Level: %s\n", preview.RiskLevel)
	summary += fmt.Sprintf("Impact: %s\n", preview.EstimatedImpact)
	summary += fmt.Sprintf("Affected Hosts: %d\n", len(preview.AffectedHosts))
	summary += fmt.Sprintf("Affected Subnets: %d\n", len(preview.AffectedSubnets))
	summary += fmt.Sprintf("Excluded Hosts: %d\n", len(preview.ExcludedHosts))

	for _, host := range preview.AffectedHosts {
		summary += fmt.Sprintf("  - %s\n", host)
	}
	if preview.RequiresApproval {
		summary += "APPROVAL REQUIRED: Execute dry-run first\n"
	}
	return summary
}

type BlastRadiusService struct {
	dryRun *DryRunMode
}

func NewBlastRadiusService() *BlastRadiusService {
	return &BlastRadiusService{
		dryRun: &DryRunMode{},
	}
}

func (s *BlastRadiusService) Preview(ctx context.Context, plan *attackplan.AttackPlan, mission *mission.Mission) (*BlastRadiusPreview, error) {
	return s.dryRun.Simulate(ctx, plan, mission)
}
