package tasks

import (
	"github.com/spf13/cobra"
	"gowork/lib"
	"log"
	"time"
)

var (
	name     string // 姓名
	greeting string // 问候语
)

var DemoJob = &cobra.Command{
	Use:   "demo",
	Short: "这是一个job的Demo的简介",
	Long: `这是一个job的Demo的详细信息这是一个job的Demo的详细信息
	
	2. 这是一个job的Demo的详细信息这是一个job的Demo的详细信息
`,
	Example: "cogo demo --name 'shugen' --greeting'welcome'",
	Run:     demoRun, //business logic
	//RunE: demoRunE, //or you can use RunE, it require your function return a error.
	PostRun: postRun, // after job run finish, some notifications you can send out.
}

func init() {
	DemoJob.PersistentFlags().StringVarP(&name, "name", "n", "cogo", "the name you want to greeting")
	DemoJob.PersistentFlags().StringVarP(&greeting, "greeting", "g", "welcome!~", "the sentence you want to greeting")
}

func postRun(cmd *cobra.Command, args []string) {
	lib.SendToUs(cmd.Use + "执行完成")
}

func demoRun(_ *cobra.Command, _ []string) {
	timer := time.After(5 * time.Second)
	go func() {
		for {
			log.Println(greeting, name)
			time.Sleep(time.Second * 1)
		}
	}()
	<-timer
}
