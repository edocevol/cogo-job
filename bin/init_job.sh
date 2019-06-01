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
