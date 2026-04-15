package analysis

func HasMeaningfulChange(result AnalysisResult) bool {
	return len(result.ChangedDependencies) > 0
}
