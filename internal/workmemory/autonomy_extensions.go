package workmemory

var builtinFlowAutonomyExtensions = []flowAutonomyExtension{
	communicationAssistFlowExtension{},
	textQualityFlowExtension{},
}

func flowAutonomyExtensions() []flowAutonomyExtension {
	return append([]flowAutonomyExtension(nil), builtinFlowAutonomyExtensions...)
}
