package attackplan

func RankPathsByRisk(graph AttackGraph) []RankedPath {
	return nil
}

func CalculatePathScore(path RankedPath) float64 {
	return path.Confidence
}
