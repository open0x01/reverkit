# reverkit demo

+ [server](./server) 一个可以直接生产环境使用的 server 实现
+ [client](./client_test) 一个简单的诠释 client 用法的包

## 本地快速体验

1. 进入 server 执行
   ```
   go run main.go --http-listen 127.0.0.1:1111 --token 123 --dbpath ./data.db
   ```
2. 执行 client
   ```
   go run main.go --server-addr http://127.0.0.1:1111 --token 123
   ```