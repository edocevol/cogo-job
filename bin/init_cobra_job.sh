#!/usr/bin/env bash
root_dir=$(cd "$(dirname "$0")"; cd ..; pwd)

#利用go generate生成jobMap.go文件
echo "---------------go generate JobCommandInit.go---------------"

dest_file=$root_dir/job/JobCommandInit.go
#更改为文件权限为允许读写
chmod 600 $dest_file

cat  > $dest_file <<EOD
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
EOD

#创建jobMap
files=`cd $root_dir/job/internal/tasks;ls | grep Job.go| grep -v BaseJob`
for filename in $files
do
    jobName=`basename $filename .go`

    cat >> $dest_file <<EOD
    	bake.AddCommand(tasks.${jobName})
EOD

done

echo "}" >> $dest_file

#格式化jobMap.go文件
go fmt $dest_file

#更改文件为只读
chmod 400 $dest_file

echo "---------------go generate success!---------------"
