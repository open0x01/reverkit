## 反连平台服务端


```bash
$ go run main.go -h
NAME:
   reverkit - A reverse service for ai.*

USAGE:
    [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --http-listen value  start a http reverse service, eg. 127.0.0.1:1111
   --token value        reverse service token
   --dbpath value       set the db path
   --config FILE        load config from FILE
   --help, -h           show help
```

注意:
+ `http-listen`、`token`、`dbpath` 这三个要么在命令行指定，要么在配置文件指定，是一定需要的。
+ token 需要保持服务端与客户端一致才可正常通信
+ db 是一个 pure go 的 kv 实现，无需第三方组件

## 从命令行直接指定

```
./server --http-listen 0.0.0.0:1111 --token complex_token --dbpath ./data.db
```

## 从配置文件指定

```yaml
# config.yaml
token: "complex_token"
db_path: ./data.db
http_listen: 0.0.0.0:1111
```

```
./server --config ./config.yml
```
