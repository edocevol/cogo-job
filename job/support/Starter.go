package support

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gowork/lib"
	"html/template"
	"log"
	"os"
	"path/filepath"
)

// 脚手架
var Starter = &cobra.Command{
	Use:   "starter",
	Short: "Start a new Job quickly",
	Long:  `Start a new Job quickly`,
	RunE:  startFunc,
}

var starterOptions struct {
	Name  string
	Short string
	Long  string
}

func userInput(tips string) string {
	// 控制输出颜色
	fmt.Printf("\n %c[%d;%d;%dm%s%c[0m", 0x1B, 0, 1, 31, tips, 0x1B)
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	return input.Text()
}
func startFunc(_ *cobra.Command, _ []string) error {
	for {
		starterOptions.Name = userInput("Input new job Name, upper camel case(like `HelloWorldJob`)>>")
		if "Job" != starterOptions.Name[len(starterOptions.Name)-3:] {
			fmt.Println("wrong case of name")
			continue
		}
		break
	}

	starterOptions.Short = userInput("Input the short description of your job, default is your job name>>")
	if starterOptions.Short == "" {
		starterOptions.Short = starterOptions.Name
	}

	starterOptions.Long = userInput("Input the detail description of your job, default is your job name>>")
	if starterOptions.Long == "" {
		starterOptions.Long = starterOptions.Name
	}

	templateObj, err := template.New(starterOptions.Name).Parse(templateCode)
	if err != nil {
		return errors.Wrap(err, "parse code template error")
	}
	codeFilePath := filepath.Join("./job/internal/tasks", starterOptions.Name+".go")
	if lib.CheckPathExist(codeFilePath) {
		log.Fatal("code file is already exists")
	}
	codeFile, err := os.OpenFile(codeFilePath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal("create code file failed", err.Error())
	}
	defer codeFile.Close()

	templateObj.Execute(codeFile, map[string]interface{}{
		"name":      starterOptions.Name,
		"shortDesc": starterOptions.Short,
		"longDesc":  starterOptions.Long,
		"lowerName": lib.Lcfirst(starterOptions.Name),
	})

	fmt.Println("Congratulation!\t File code path:", codeFilePath)

	fmt.Printf("\n %c[%d;%d;%dm%s%c[0m\n", 0x1B, 1, 1, 33, "If you want to use your job, run `go generate` first.", 0x1B)
	fmt.Printf("\n %c[%d;%d;%dm%s%c[0m\n", 0x1B, 1, 1, 33, "If you want to use your job, run `go generate` first.", 0x1B)
	fmt.Printf("\n %c[%d;%d;%dm%s%c[0m\n", 0x1B, 1, 1, 33, "If you want to use your job, run `go generate` first.", 0x1B)

	return nil
}

var templateCode = `// This file is auto generated.
package tasks

import (
	"fmt"
	"github.com/spf13/cobra"
)

var {{.name}} = &cobra.Command{
	Use:     "{{.name}}",
	Short:   "{{.shortDesc}}",
	Long:    "{{.longDesc}}",
	Example: "./gobake {{.name}}", //TODO: write how to run your job
	RunE:    {{.name}}Func,
}

// Job需要使用的参数，为了避免同包下参数重复定义，用Job名包裹起来
// TODO: If you need define local variables, implement your code here
//var {{.lowerName}}Options struct {
//	age int
//	name string
//}

// This is **cobra.Command** runnable method, you can change method name,
// but avoid name conflict
func {{.name}}Func(_ *cobra.Command, _ []string) error {
	//TODO: Implements your own job logic
	fmt.Println("Hello World")
	return nil
}
`
