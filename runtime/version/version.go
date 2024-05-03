package version

import "fmt"

const (
	DeployerMajor = 0
	DeployerMinor = 1
)

var (
	DeployerVersion = SemVer{DeployerMajor, DeployerMinor, 0}
)

type SemVer struct {
	Major int
	Minor int
	Patch int
}

func (v SemVer) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}
