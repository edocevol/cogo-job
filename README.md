## 背景

之前团队中对Job任务的编写多采用脚本语言如PHP、Ruby等编写逻辑，然后利用crontab和supervisor进行作业的调度和检测。

近期需要对云上使用的访问日志中记录的流量信息，进行统计，按照原先的Job方案，采用Laravel进行了Job的编写，本地测试全部Ok，放到线上进行测试，拉取最近一个月的访问日志文件后，很长的时间Job都没有执行完成，原先的Laravel Job在一定程度上可以使用fork子线程的方式实现多线程，但也有很多限制，比如不能再for循环里创建线程。因此，考虑线上服务器上处理大量的访问日志文件时，不能充分利用服务器的多核处理能力；线上有部分接口已经采用Go进行同步开发，用Go编写Job具有一定的可行性。因此，经过一段时间的讨论，输出一个简单可自由拓展的Go Job方案。

## 调度

首先，最先遇到的一个问题是用Go写好了Job的逻辑，如何实现Job的调度？

可选的方案有两种：

*   采用社区开源的Go库cron对Job进行自调度
*   采用linux crontab对Job进行调度

针对第一种方案，[cron](https://github.com/robfig/cron)（github链接）可以实现秒级的定时任务；每个任务都是一个Task（简单来说就是一个func）；每个Job注册之后，会在独立的协程中执行。这样的方案明显有以下几个特点：

*   可以实现秒级的Job调度，linux crontab只能实现分钟级的调度；
*   可以充分利用协程的优势
*   由于go routine无法对子go routine的生命周期进行管理，因此，Job一旦启动，cron库没有能力终止某个Job，只能全部终止

针对第二种方案，可以很好的利用linux crontab的能力，减少对组件的依赖，也方便及时终止Job，但这就要求我们能够对Job进行拆分，每个crontab表达式注册一个Job任务。

经过调研和分析，决定对Job采用类似于Laravel的Job实现方案。

## 反射实现分发

Laravel等框架利用了PHP的动态语言的特性，可通过诸如new  `XXXX`::class() 的方式实现根据一个字符串找到其对应得Class，然后实例化一个对象出来，但是，Go语言做不到这样的。

### 规则定义

同大部分语言一样，Go提供了强大的反射功能，利用反射，我们可以获取某个对象（type Xxx struct）下定义的方法、方法的签名等等。因此，我们可以根据定义如下规则：

*   定义基础Job结构体，type BaseJob interface，定义方法Run
*   所有的Job文件都定义自己的结构体，type Xxx Struct
*   所有的Job文件实现Run方法

对应得代码如下：

```golang
package gjobs

type BaseJob interface {
	Run(name string, args ...interface{})
}

```

 Job的代码示例

**注意**： Run方法的args是变参，示例中是通过切片操作得到第一个参数，`(args[1:2])[0]`可以得到第二个参数，其他依次类推，可以将字符串转成自己需要的基本类型（`int,bool等`）

```golang
package gjobs

import (
	"log"
	"time"
)

type HelloWorldJob struct {
}

func (job *HelloWorldJob) Run(jobName string, args ...interface{}) {
	if len(args) < 2 {
		log.Fatal("参数数量错误")
	}

	name := (args[0:1])[0] // 取第一个参数
	// 用timer来模拟一个需要运行30s的任务
	timer := time.After(time.Second * 6)
	select {
	case <-timer:
		break
	}
	log.Println("===============================")
	log.Println("===============================")
	log.Println("hello", name)
	log.Println("===============================")
	log.Println("===============================")
	log.Println("Job 运行完毕")
}

```

### 反射

如下面的代码所示，jobParamList是我们的Job要用到的参数列表。下面的代码，主要是四个步骤

*   查找Job的Type信息，根据Type信息才能找到方法
*   根据Type信息，找到该Job实现的Run方法
*   根据参数列表jobParamList设置Run方法的参数
*   使用Call方法调用HelloWorldJob的Run方法，并得到返回值

```golang
func execJob() {
	params := strings.Split(jobParamList, ";")
	jobObj := reflect.ValueOf(gjobs.AllJobMap[jobName])
	
	if jobObj == reflect.Zero(jobObj.Type()) {
		log.Println("找不到Job，退出执行")
		return
	}
	
	valueFunc := jobObj.MethodByName("Run")
	if valueFunc == reflect.Zero(valueFunc.Type()) {
		log.Println("找不到Job中的Run方法，退出执行")
		return
	}
	
	paramList := make([]reflect.Value, 0, len(params)+1) //多了一个jobName
	paramList = append(paramList, reflect.ValueOf(jobName))
	
	for k := range params {
		paramList = append(paramList, reflect.ValueOf(params[k]))
	}
	
	// 反射调用函数
	resultList := valueFunc.Call(paramList)
	log.Println("exec end", map[string]interface{}{
		"jobName":     jobName,
		"params":      params,
		"returnValue": resultList,
	})
	return
	
}
```

### Job发现问题

上面的代码已经能够让你明白如何能够对一个Job进行参数的设置和调用，但是有一个问题是，我们要怎么样才能通过参数的方式告诉程序要创建HelloWorldJob对象，并调用其Run方法呢。

之前说了，Go做不到php那样，因此，如何通过一个字符串来告诉main routine我们要执行哪个Job？

一个简单的方案就是维护一个map，Job的字符串名对一个Job对象。如下所示

```golang
package gjobs

//please do not edit this JobMap.go, it is auto created by go generate
var AllJobMap = map[string]interface{}{
	"HelloWorldJob": &HelloWorldJob{},
}

```

当Job的数量很少时，我们维护这个map是比较简单的，当Job的数量上升之后以及多人协作开发时，容易出现问题，因此，经过考虑，决定使用go generate 来实现对这个map文件的自动生成和维护，对应的脚本如下（如果你对go generate不了解，可以先google一下）：

```bash
#!/usr/bin/env bash
root_dir=$(cd "$(dirname "$0")"; cd ..; pwd)
dest_file=$root_dir/gjobs/JobMap.go

#清空文件内容
echo -n "" > $dest_file

#利用go generate生成jobMap.go文件
echo "---------------go generate JobMap.go---------------"

cat >> $dest_file <<EOD
package gjobs
//please do not edit this JobMap.go, it is auto created by go generate
var AllJobMap = map[string]interface{}{
EOD

#创建jobMap
files=`cd $root_dir/gjobs;ls | grep Job.go| grep -v BaseJob`
for filename in $files
do
    jobName=`basename $filename .go`
    cat >> $dest_file <<EOD
    "${jobName}": &${jobName}{},
EOD
done

echo "}" >> $dest_file

#格式化jobMap.go文件
go fmt $dest_file

echo "---------------go generate success!---------------"

```

 只需要在主入口go文件上加上

```golang
...
//go:generate ./bin/init_job.sh
// 初始化一些工作，如启动参数获取，健康检查
func init() {
}
...
```

这样的注释就可以了，在需要重新生成map文件的时候，执行一下go generate命令即可。

![go generate 会执行当前项目下的所有go generate 注释](http://cdn.wanqing520.cn/blog/Ashampoo_Snap_2019.06.01_15h40m32s_002_.png)

### 使用

go build 之后，可以使用下面的方式运行指定的Job

![go build & try run](http://cdn.wanqing520.cn/blog/Ashampoo_Snap_2019.06.01_15h38m37s_001_.png)

## 使用产线（plugin）动态加载Job

Go在打包时，可以将一个main package下的文件打包成 **.so**文件，利用plugin的特性，可以实现Job的热更新等机制。
在上面论述的基础上，简单说明一下go plugin实现Job动态更新的方案。


###  规则定义

plugin文件在构建的时候要求当前包是 `main`
为了将Job统一管理，在`gjobs`文件夹下创建一个`plugin`文件，每个`Job`在单独的文件夹下面，这里我们假设有一个`Demo`的`Job`，对应的Job实现的文件是`PluginDemoJob.go`文件。由于我们需要Job统一暴露一个公开的同名Run方法，所以需要将不同的`Job`放在不同的文件夹下，避免出现公开Run方法重复定义的问题  。



![plugin的job代码组织](http://cdn.wanqing520.cn/blog/1559375572123.png)



其中，`Demo`目录下的`PluginDemoJob`对应的代码如下：

```golang
package main

import (
	"log"
	"time"
)

func Run(jobName string, args ...interface{}) {
	if len(args) < 2 {
		log.Fatal("参数数量错误", args)
	}
	
	name := (args[0:1])[0] // 取第一个参数
	// 用timer来模拟一个需要运行30s的任务
	timer := time.After(time.Second * 6)
	select {
	case <-timer:
		break
	}
	log.Println("===============================")
	log.Println("===============================")
	log.Println("This is a go plugin job demo", name)
	log.Println("===============================")
	log.Println("===============================")
	log.Println("Job 运行完毕")
}

```
### Job发现

由于plugin是动态加载的方式，只需要提供`.so`文件的路径，就可以加载了，实现Job发现就很简单了，不需要维护映射文件。

plugin的构建很简单，和普通的go文件的构建命令的区别在与`--buildmode=plugin`这个参数。参考下图，同时plugin的构建也可以使用`-o`参数指定构建生成的`.so`文件的输出路径，为了简单起见，本文所有的操作都是将plugin的构建结果放在项目的根目录下。
```
go build --buildmode=plugin ./gjobs/plugin/Demo/PluginDemoJob.go
```
 上面的命令指定了构建的模式是生成plugin, 指定了要构建的文件路径（需要是一个完整的路径）
![构建plugin](http://cdn.wanqing520.cn/blog/1559375375719.png)
会在运行上面命令的地方生成一个PluginDemoJob.so的文件( **可以在build子命令后面使用-o指定输出路径** )

下面看一下如何加载.so文件

```golang
func execJobWithPluginWay() {
	plugin, err := plugin.Open("./" + jobName + ".so")
	if err != nil {
		log.Fatal("找不到Job")
		return
	}
	// 查找是否有我们统一要求保留出来的Run方法
	runMethod, err := plugin.Lookup("Run")
	if err != nil {
		log.Fatal("找不到Job的Run方法", err)
		return
	}
	// 将我们通过main程序传进来的字符串参数转成interface{}变参
	params := strings.Split(jobParamList, ";")
	paramList := make([]interface{}, 0, len(params))
	for k := range params {
		paramList = append(paramList, params[k])
	}
	
	// 类型断言
	run := runMethod.(func(string, ...interface{}))
	// 执行Run方法
	run(jobName, paramList...)
	
	log.Println("exec end", map[string]interface{}{
		"jobName": jobName,
		"params":  params,
	})
	return
}

```



## cobra的方式


### 目录结构说明
cobra在Golang生态圈中有很多开源项目都在使用，比如`docker`、`k8s`等等，能够快速的实现命令行程序的开发，方便我们实现参数解析。

关于cobra的详细介绍，可以参考[cobra@github](https://github.com/spf13/cobra.

先看一下代码结构

![enter description here](http://cdn.wanqing520.cn/blog/1559378077677.png)

####  `cogo.go`
此文件是cobra版本的Job实现的主入口函数，主要是执行正在的Job命令，就几行代码。
```golang
package main

import (
	"gowork/job"
	"os"
)

//go:generate ./bin/init_cobra_job.sh

func main() {
	if err := job.BakeJob.Execute(); err != nil {
		os.Exit(-1)
	}
}

```


#### `job/CoJob.go`

此文件的作用是自定义了cobra的主命令，在本文中，主项目的名字为`cogo`，其定义和描述信息如下
```golang
var CogoJob = &cobra.Command{
	Use:               "cogo",
	Short:             "cogo, a job framework base on cobra",
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	Long:              "基于cobra的Golang Job实现方案",
	PersistentPreRun:  preRun,
}
```
上面的定义的说明如下（其他更多参数的使用，可以参看cobra的文档）
- `Use` 定义当前命令的使用方式
-  `Short` 定义当前命令的简介信息
-  `Long` 定义当前命令的详细介绍
-  `PersistentPreRun`定义在执行当前命令的前置执行函数

从下图可以看到我们定义的参数已经显示出来了
![enter description here](http://cdn.wanqing520.cn/blog/1559378588612.png)

**定义当前命令需要的参数及参数绑定**

为了方便进行参数管理，我们将当前命令需要的参数放在一个struct中。
```golang
var coJobOptions = struct {
	pprofHost string // pprof 性能监控绑定的IP地址，正式环境尽量只允许本地访问
	pprofPort int    // pprof 性能监控随机获取一个可用的端口号，正式环境尽量只允许本地访问
	cpuNum    int    //指定cpu核数
	//TODO 在这里可以继续声明log目录，配置文件的路径等待
	//log_dir string
	//cfg_file string
}{}
```

在当前文件的`init`函数中，可以实现对传入参数的解析

```golang
func init() {
	//NOTE 这里不能使用h作为参数，-h参数为help占用
	CogoJob.PersistentFlags().StringVarP(&coJobOptions.pprofHost, "host", "m", "0.0.0.0", "pprof server ip")
	CogoJob.PersistentFlags().IntVarP(&coJobOptions.cpuNum, "cpu_num", "p", 1, "cpu num")
}
```

如果遇到需要长时间运行的Job，我们可能需要关注goroutine的运行情况，可以在job启动的时候，启动pprof服务(当前命令的PersistentPreRun方法)
需要注意的是，在线上运行的时候，避免将pprof暴露在外网。

```golang
//preRun函数内的代码也可以放在init函数中进行
//      TODO 1. 初始化日志配置
//      TODO 2. 初始化数据库相关配置
//      TODO 3. 初始化Redis相关配置
//      TODO 4. etc...
func preRun(cmd *cobra.Command, args []string) {
	defer func() {
		<-pprofStarted
	}()

	// 开启pprof性能监控
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println("发生panic")
				lib.SendToUs("Job发生panic")
			}
		}()
		// 这些命令不需要启动pprof监控
		if cmd.Use == "version" || cmd.Use == "starter" {
			pprofStarted <- true
			return
		}
		address := fmt.Sprintf("%s:%d", coJobOptions.pprofHost, coJobOptions.pprofPort)
		mux := http.NewServeMux() //创建一个http ServeMux实例
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.HandleFunc("/check", HealthCheckHandler)
		mux.HandleFunc("/pid", GetThreadIdHandler)

		log.Println("server pprof run on: ", address)
		pprofStarted <- true

		if err := http.ListenAndServe(address, mux); err != nil {
			log.Fatal("pprof error: ", err)
		}
	}()

}

```

#### `support/Version.go`
此文件是一个对`version`命令的简单实现，通过使用`cogo version`命令，可以知道当前的Job的版本信息，方便我们及时处理问题，目前的实现比较简单，大家可以参考`go generate`的方式，结合CI等步骤，对这里的version进行拓展。

#### `support/Starter.go`

该文件是提供给我们快速建立一个符合规范、快速上手的脚手架工具，工具定义了子命令需要参照的标准，通过执行`cogo starter`命令能够快速的创建一个Job。下面对该文件进行简单说明。

下方的代码中的`Use`声明当前子命令为`starter`，子命令执行的函数为`startFunc`
```golang
// 脚手架
var Starter = &cobra.Command{
	Use:   "starter",
	Short: "Start a new Job quickly",
	Long:  `Start a new Job quickly`,
	RunE:  startFunc,
}
```

`startFunc`的主要逻辑如下

- 请求用户需要一个大驼峰式的Job名称
- 请求用户输入Job的简短描述，便于在`cogo help`时查看
- 请求用户输入Job的详细描述，便于在`cogo  xxxJob`时查看
- 根据用户的输入信息，利用go的模板技术，在`job/internal/tasks`目录下生成Job文件


![enter description here](http://cdn.wanqing520.cn/blog/1559379734393.png)

最终生成的文件形式如下，并且在控制台上也有提示，只需要运行`go generate`就可以使用`go build`构建代码了

```golang
// This file is auto generated.
package tasks

import (
	"fmt"
	"github.com/spf13/cobra"
)

var TestJob = &cobra.Command{
	Use:     "TestJob",
	Short:   "TestJob",
	Long:    "TestJob",
	Example: "./gobake TestJob", //TODO: write how to run your job
	RunE:    TestJobFunc,
}

// Job需要使用的参数，为了避免同包下参数重复定义，用Job名包裹起来
// TODO: If you need define local variables, implement your code here
//var testJobOptions struct {
//	age int
//	name string
//}

// This is **cobra.Command** runnable method, you can change method name,
// but avoid name conflict
func TestJobFunc(_ *cobra.Command, _ []string) error {
	//TODO: Implements your own job logic
	fmt.Println("Hello World")
	return nil
}

```

从下图的可以看到，运行`go generate`和`go build`之后，如下图中的绿色高亮区域所示，我们新建的TestJob子命令已经可以使用了。
![enter description here](http://cdn.wanqing520.cn/blog/1559379931646.png)

运行，我们的Job就可以运行了
![enter description here](http://cdn.wanqing520.cn/blog/1559380053182.png)

### `go generate`干了啥

上面的几张图演示了我们的代码目录组织，以及如何快速添加一个新的Job，并且已经对Starter的实现进行了说明，下面就接着说一个`go generate`干了啥

我们在main函数上注策了一个`go generate`
![enter description here](http://cdn.wanqing520.cn/blog/1559380227809.png)

告诉go generate 执行时，执行的命令为`./bin/init_cobra_job.sh`文件。`bin/init_cobra_job.sh`的主要内容是生成下面这个文件

遍历`job/internal/tasks/`目录下的所有`*Job.go`文件，将改文件内的子命令以`AddCommand`函数的方式添加到我们在`CoJob.go`中定义的CogoJob主命令中。



![enter description here](http://cdn.wanqing520.cn/blog/1559380375732.png)
在`CoJob.go`的`init`函数中，我们实现了对自动生成的只读文件`JobCommandInit.go`文件中定义的`AutoInitJob`方法的调用
```
	CogoJob.AddCommand(support.Version)
	CogoJob.AddCommand(support.Starter)

	AutoInitJob(CogoJob)
```

至此，基于cobra实现Job的方案已经讲解完成。
