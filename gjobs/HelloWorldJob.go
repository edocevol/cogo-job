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
