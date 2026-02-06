package ssm

import "github.com/sachamama/sacha/internal/ssm"

// parametersLoadedMsg is sent when the initial parameter listing completes.
type parametersLoadedMsg struct {
	params    []ssm.Parameter
	nextToken *string
	err       error
}

// moreParametersLoadedMsg is sent when additional parameters are loaded (lazy loading).
type moreParametersLoadedMsg struct {
	params    []ssm.Parameter
	nextToken *string
	err       error
}

// parameterDetailMsg is sent when a single parameter's full details are fetched.
type parameterDetailMsg struct {
	param *ssm.Parameter
	name  string // name to match against current cursor
	err   error
}
