# mq-listener

## 配置文件说明

```yaml

name: mqlistener        # 指定实例名，作为日志文件名和消费者消息key的一部分：loggerTopic + "_" + strings.ToUpper(name)
debug: true             # 主要影响日志级别 
broker:
  host: 127.0.0.1
  port: 7270
  token: 1111111
  messageEncrypt: false       # 是否加密消息
  messageEncryptPlan: token   # 消息加密方案token/no,仅是否加密消息=true时有效
loggerTopic: MQ_LISTENER      # 系统调用执行结果输出topic 

listeners: # 要监听的topic，一个列表，修改后重启生效
  - topic: DNS_REPORT   # 监听的topic名称
    monitor: # 系统调用触发条件
      where: value      # 触发位置，取值 key/value（不区分大小写）
      onEvent: updated  # 引起触发的判断条件，取值 received/updated（不区分大小写）
      fieldName: ip     # 当value内部此字段发生变化时发起系统调用，仅 where==value 有效
      cmd: echo         # 系统调用命令，此命令固定有一个参数，当 where==key 时为key的值，where==value时为fieldName的值
    key:
      name: time        # key名称，用于终端图形显示
      zhCN: 消息创建时间
      type: string      # 类型，固定string
    value: # topic-value的描述
      fields: # value内存在的全部字段信息
        - name: domain  # 第一个字段名
          zhCN: 域名
          type: string  # 字段类型，支持：string/int/float/bool/map，程序会依据此类型来反序列化value
        - name: ip
          zhCN: 域名解析的目标IP地址
          type: string
      unmarshalType: json     # value 的反序列化方式，仅支持json

  - topic: DNS_CHANGED
    monitor:
      where: key
      onEvent: received
      fieldName:
      cmd: echo
    key:
      name: time
      zhCN: 变动时间
      type: string
    value:
      fields:
        - name: old
          zhCN: 旧IP地址
          type: string
        - name: new
          zhCN: 新IP地址
          type: string
      unmarshalType: json

```

### 示例说明

```yaml
  - topic: DNS_CHANGED
    monitor:
      where: key
      onEvent: received
      fieldName:
      cmd: echo
    key:
      name: time
      zhCN: 变动时间
      type: string
    value:
      fields:
        - name: old
          zhCN: 旧IP地址
          type: string
        - name: new
          zhCN: 新IP地址
          type: string
      unmarshalType: json
```

- 监听名为`DNS_CHANGED`的topic
- 当收到一条消息时（`onEvent: received`）即引起系统调用，系统调用命令为`echo key-value`（`cmd: echo`）+（`where: key`）
- 此value有`2`个字段，全部为`string`类型

```json
{
  "old": "",
  "new": ""
}
```

### monitor 示例

```yaml
monitor:
  where: value
  onEvent: updated
  fieldName: ip
  cmd: echo
```

- 当`topic-value`内`ip`字段的值发生变化`update`时引起系统调用，系统调用命令为`echo value`;

## Logger

当配置的某一个`listener`被触发后将执行结果上报到mq

- topic: loggerTopic
- key: loggerTopic + "_" + strings.ToUpper(name)
- value:

```json
{
  "listen_topic": "",
  "on_event": "updated",
  "where": "value",
  "field_name": "ip",
  "cmd": "echo 127.0.0.1",
  "result": "SUCCEED",
  "output": "127.0.0.1"
}


```