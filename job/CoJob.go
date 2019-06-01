package job

import (
	"fmt"
	"gowork/job/support"
	"gowork/lib"
	"log"
	"net/http"
	"net/http/pprof"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
)

var pprofStarted = make(chan bool, 1)
var coJobOptions = struct {
	pprofHost string // pprof 性能监控绑定的IP地址，正式环境尽量只允许本地访问
	pprofPort int    // pprof 性能监控随机获取一个可用的端口号，正式环境尽量只允许本地访问
	cpuNum    int    //指定cpu核数
	//TODO 在这里可以继续声明log目录，配置文件的路径等待
	//log_dir string
	//cfg_file string
}{}

var BakeJob = &cobra.Command{
	Use:               "./cogo",
	Short:             "cogo, a job framework base on cobra",
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	Long:              "基于cobra的Golang Job实现方案",
	PersistentPreRun:  preRun,
}

func init() {
	//NOTE 这里不能使用h作为参数，-h参数为help占用
	BakeJob.PersistentFlags().StringVarP(&coJobOptions.pprofHost, "host", "m", "0.0.0.0", "pprof server ip")
	BakeJob.PersistentFlags().IntVarP(&coJobOptions.cpuNum, "cpu_num", "p", 1, "cpu num")

	BakeJob.AddCommand(support.Version)
	BakeJob.AddCommand(support.Starter)

	AutoInitJob(BakeJob)

	cpuMaxnum := runtime.NumCPU()
	if coJobOptions.cpuNum == 0 || coJobOptions.cpuNum >= cpuMaxnum {
		coJobOptions.cpuNum = cpuMaxnum
	}
	runtime.GOMAXPROCS(coJobOptions.cpuNum)

	var err error
	coJobOptions.pprofPort, err = lib.PortCanIUse()
	if err != nil {
		// 这里可以采取不fatal的方式，在PreRun中不启动的pprof也可以
		log.Fatal("获取pprof端口失败")
	}
}

//健康检测http://localhost:30051/check
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"active":true}`))
}

func GetThreadIdHandler(w http.ResponseWriter, r *http.Request) {
	response := fmt.Sprintf(`{"code":0, "msg": "success", "data": {"pid": %d}}`, syscall.Getpid())
	w.WriteHeader(200)
	w.Write([]byte(response))
}

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
