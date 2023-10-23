package config

import (
	"github.com/Chendemo12/functools/environ"
	"strings"
	"sync"
)

var (
	only *sync.Once
	conf *AppConfig
)

func init() {
	only = &sync.Once{}
	conf = &AppConfig{
		Name:  "listener",
		Debug: false,
		Broker: &BrokerConf{
			Host: "127.0.0.1",
			Port: "7270",
		},
		Listeners: make([]*Listener, 0),
	}
}

func Get() *AppConfig { return conf }

// LoadConfig 从配置文件中加载配置文件
func LoadConfig(filepath string) (*AppConfig, error) {
	var err error

	only.Do(func() {
		err = environ.ViperParse(conf, filepath)
		conf.init()
	})

	return conf, err
}

// AppConfig 全局配置文件
type AppConfig struct {
	Name        string      `json:"name" yaml:"name" description:"程序名,同样作为日志名"`
	Debug       bool        `json:"debug" yaml:"debug" description:"调试模式"`
	Broker      *BrokerConf `json:"broker" yaml:"broker" description:"MQ配置参数"`
	LoggerTopic string      `json:"logger_topic" yaml:"loggerTopic" description:"执行结果投递topic"`
	Listeners   []*Listener `json:"listeners" yaml:"listeners" description:"监听的分区"`
}

func (c *AppConfig) Topics() []string {
	topics := make([]string, len(c.Listeners))
	for i, listener := range c.Listeners {
		topics[i] = listener.Topic
	}

	return topics
}

func (c *AppConfig) init() {
	// 初始化空值
	for _, listener := range c.Listeners {
		if listener.Monitor == nil {
			listener.Monitor = &Callable{
				Where:     NotCallBy,
				FieldName: "",
				Cmd:       "",
			}
		}
		if listener.Value == nil {
			listener.Value = &Value{
				Fields:        make([]*FieldInfo, 0),
				UnmarshalType: JsonUnmarshalType,
			}
		}
		listener.Monitor.lock = &sync.Mutex{}
	}
}

type BrokerConf struct {
	Host               string `json:"mq_host" yaml:"host" description:"MQ地址"`
	Port               string `json:"mq_port" yaml:"port" description:"MQ端口号"`
	Token              string `json:"token" yaml:"token"`
	MessageEncrypt     bool   `json:"message_encrypt" yaml:"messageEncrypt" description:"是否加密消息"`
	MessageEncryptPlan string `json:"message_encrypt_plan" yaml:"messageEncryptPlan" description:"加密方案 Token/no"`
}

type Listener struct {
	Topic   string     `json:"topic" yaml:"topic" description:"分区名"`
	Monitor *Callable  `json:"monitor" yaml:"monitor" description:"系统调用"`
	Key     *FieldInfo `json:"key" yaml:"key" description:"消息的KEY"`
	Value   *Value     `json:"value" yaml:"value" description:"消息的值"`
}

// NewValueInstance 创建副本
func (c Listener) NewValueInstance() map[string]any {
	m := make(map[string]any)
	for _, field := range c.Value.Fields {
		switch field.Type {
		case BoolFieldType:
			m[field.Name] = false
		case StringFieldType:
			m[field.Name] = ""
		case IntFieldType:
			m[field.Name] = 0
		case FloatFieldType:
			m[field.Name] = float32(0.0)
		case MapFieldType:
			m[field.Name] = make(map[string]any)
		default:
			m[field.Name] = ""
		}
	}

	return m
}

// NeedUnmarshalValue 是否需要反序列化值
// 目前仅支持 JSON 反序列化
func (c Listener) NeedUnmarshalValue() bool {
	return len(c.Value.Fields) != 0 &&
		strings.ToLower(string(c.Value.UnmarshalType)) == string(JsonUnmarshalType)
}

// CallByValue 是否需要在值变化时发起系统调用
func (c Listener) CallByValue() bool {
	if c.Monitor.ByValue() {
		for _, field := range c.Value.Fields {
			if field.Name == c.Monitor.FieldName {
				return true
			}
		}
	}
	return false
}

// CallByKey 是否需要在KEY变化时发起系统调用
func (c Listener) CallByKey() bool { return c.Monitor.ByKey() }

type CallByWhat string

const (
	NotCallBy   CallByWhat = "NOT"
	CallByKey   CallByWhat = "key"
	CallByValue CallByWhat = "value"
)

type CallOnEvent string

const (
	CallOnReceived CallOnEvent = "received"
	CallOnUpdated  CallOnEvent = "updated"
)

// Callable 允许在参数发生变化时，发起一个系统调用
type Callable struct {
	Where   CallByWhat  `json:"where" yaml:"where" description:"触发依据"`
	OnEvent CallOnEvent `json:"on_event" yaml:"onEvent" description:"触发时机"`
	// 如果触发事件为value,若此值不为空，则进一步判断此字段是否引发变化，否则判断value的原始字节序来判断
	// 且此值必须是 Value.Fields 之一
	FieldName string `json:"field_name" yaml:"fieldName" description:"事件触发字段"`
	// 触发命令。linux命令或一个shell脚本路径
	// 此命令固定接收一个参数，若 Where=key，则为key的值，若 Where=value,则为 FieldName 的值
	Cmd  string      `json:"cmd" yaml:"cmd" description:"触发命令"`
	lock *sync.Mutex // 系统调用执行锁
}

func (c Callable) Lock()   { c.lock.Lock() }
func (c Callable) UnLock() { c.lock.Unlock() }

func (c Callable) ByKey() bool {
	return strings.ToLower(string(c.Where)) == string(CallByKey)
}

// ByValue 当值发生变化时触发调用
func (c Callable) ByValue() bool {
	return strings.ToLower(string(c.Where)) == string(CallByValue)
}

// Cmds 命令及参数
//func (c Callable) Cmds() []string { return strings.Split(c.Cmd, " ") }

type ValueUnmarshalType string

const (
	JsonUnmarshalType ValueUnmarshalType = "json"
)

// Value topic 内值的描述
type Value struct {
	Fields        []*FieldInfo       `json:"fields" yaml:"fields" description:"全部字段"`
	UnmarshalType ValueUnmarshalType `json:"unmarshal_type" yaml:"unmarshalType" description:"此值的序列化方式"`
}

type FieldType string

const (
	StringFieldType FieldType = "string"
	IntFieldType    FieldType = "int"
	FloatFieldType  FieldType = "float"
	BoolFieldType   FieldType = "bool"
	MapFieldType    FieldType = "map"
)

// FieldInfo 字段描述
type FieldInfo struct {
	Name string    `json:"name" yaml:"name" description:"字段名称"`
	ZhCN string    `json:"zh_cn" yaml:"zhCN" description:"字段中文解释"`
	Type FieldType `json:"type" yaml:"type" description:"字段的类型"`
}
