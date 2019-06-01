package main

import (
	"context"
	"flag"
	"fmt"
	"gowork/gjobs"
	"gowork/lib"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"plugin"
	"reflect"
	"strings"
	"syscall"
	"time"
)

var host string
var port int

var jobWaitTime time.Duration //每个job超时限制1s or 1m
var jobName string            //要运行的job名称
var jobParamList string       //要运行的job的参数
//go:generate ./bin/init_job.sh
// 初始化一些工作，如启动参数获取，健康检查
func init() {
	flag.StringVar(&host, "host", "0.0.0.0", "app listen host")
	flag.StringVar(&jobName, "job_name", "HelloWorldJob", "job name")
	flag.StringVar(&jobParamList, "job_params", "", "job param")
	flag.Parse()
	jobWaitTime = time.Duration(time.Second * 60)

	port, err := lib.PortCanIUse() // 获取一个随机可以使用的端口号
	if err != nil {
		log.Fatalln("get validate port error: ", err)
		return
	}
	// 开启pprof性能监控
	go func() {
		defer func() {}()

		address := fmt.Sprintf("%s:%d", host, port)
		log.Println("server pprof run on: ", address)
		mux := http.NewServeMux() //创建一个http ServeMux实例
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.HandleFunc("/pid", GetThreadIdHandler)

		if err := http.ListenAndServe(address, mux); err != nil {
			log.Println("pprof error: ", err)
		}
	}()
}

// 通过反射找到对应的Job执行信息
func main() {
	log.Printf("当前服务的pid: %d", syscall.Getpid())
	ctx, cancel := context.WithTimeout(context.Background(), jobWaitTime) //Job的超时控制

	go func() {
		defer cancel()
		if _, ok := gjobs.AllJobMap[jobName]; !ok {
			// 如果没有使用在JobMap中没有映射，尝试通过插件加载的方式寻找
			execJobWithPluginWay()
		} else {
			execJob()
		}
		log.Println("job exec finished")
	}()

	//平滑退出信号量
	ch := make(chan os.Signal, 1)
	go func() {
		for {
			select {
			// 超时退出
			case <-ctx.Done():
				ch <- syscall.SIGQUIT
				break
			default:
				log.Println(jobName, "still running with pid: ", syscall.Getpid())
				break
			}
			time.Sleep(time.Second * 5)
		}
	}()

	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGHUP)
	sig := <-ch

	log.Println("signal: ", sig.String())
	log.Println("main routine will exit...")
}

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

func GetThreadIdHandler(w http.ResponseWriter, r *http.Request) {
	response := fmt.Sprintf(`{"code":0, "msg": "success", "data": {"pid": %d}}`, syscall.Getpid())
	w.WriteHeader(200)
	w.Write([]byte(response))
}

//每个job执行
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
