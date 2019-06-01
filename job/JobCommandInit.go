//please do not edit this JobMap.go, it is auto created by go generate
//please do not edit this JobMap.go, it is auto created by go generate
//please do not edit this JobMap.go, it is auto created by go generate
package job

import (
	"github.com/spf13/cobra"
	"gowork/job/internal/tasks"
)

//please do not edit this JobMap.go, it is auto created by go generate
//please do not edit this JobMap.go, it is auto created by go generate
//please do not edit this JobMap.go, it is auto created by go generate
func AutoInitJob(bake *cobra.Command) {
	bake.AddCommand(tasks.DemoJob)
}
