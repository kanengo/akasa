package pkg

type StackFrame struct {
	Func   string `json:"func"`
	Source string `json:"source"`
	Line   int    `json:"line"`
}

func ErrorStack(err error) error {
	return nil

}
