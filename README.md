# 反连平台 api

这里主要讲代码中的 api 使用

**如果需要查看产品集成相关的用法, 请进入
[cmd](./_example) 文件夹**

## features

触发方式
 - http 访问 （`url := unit.GetVisitURL()`）
 - rmi 访问 (`url := unit.GetRmiURL()`)
 
回调方式
 - 异步回调函数
 - 同步阻塞调用

## 基础结构

该反连服务分为两部分：server 和 client


## group 和 unit

一个 unit 对应一个触发方式。一个 group 可以包含多个 unit。

在编写检测逻辑时，常常会遇到一种情况是同一个漏洞有多种利用方式，如果把
每一种方式都发包检测，那么当触发多次时就会报出多个重复漏洞，这时候就可以
把同一个目标用一个 group 包裹, 这样当触发时只会报出一个漏洞。

```go
unitGroup := reverse.NewUnitGroup()
for _, payload := range XXEPayloads {
    unit, err := reverse.RegisterWithGroup("", unitGroup)
    if err != nil {
        break
    }
	
    reverseURL := unit.GetVisitURL()
	// use reverseURL
	// ...

	// 定义触发逻辑
    unit.OnVisit(func(event *reverse.Event) error {
		vuln := newVuln()
        vuln.Add("remote_request", event.Request)
        vuln.Add("remote_ip", event.RemoteAddr)
		vuln.Add("payload", "test")
		outputVuln(vuln)
        return nil
    })
}
```

同步等待代码实例

```go
syncUnit, _ := reverse.Register("")
reverseURL := unit.GetVisitURL()

// send reverseURL
// ... 

// 等待 url 被触发最多 5s 种
syncUnit.Wait(5 * time.Second)
```