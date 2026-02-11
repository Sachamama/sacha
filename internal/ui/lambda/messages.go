package lambda

import "github.com/sachamama/sacha/internal/lambda"

// functionsLoadedMsg is sent when function listing completes.
type functionsLoadedMsg struct {
	functions []lambda.Function
	nextToken *string
	err       error
}

// moreFunctionsLoadedMsg is sent when additional functions are loaded.
type moreFunctionsLoadedMsg struct {
	functions []lambda.Function
	nextToken *string
	err       error
}

// functionDetailsMsg is sent when function details are fetched.
type functionDetailsMsg struct {
	details      *lambda.FunctionDetails
	functionName string
	err          error
}

// allFunctionsLoadedMsg is sent when all remaining functions are loaded.
type allFunctionsLoadedMsg struct {
	functions []lambda.Function
	err       error
}
