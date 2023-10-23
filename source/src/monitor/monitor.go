package monitor

import (
	"context"
	"fmt"
	"gitee.com/chendemo12/mqlistener/src/config"
	"github.com/Chendemo12/fastapi-tool/helper"
	"github.com/Chendemo12/fastapi-tool/logger"
	"github.com/Chendemo12/functools/zaplog"
	"github.com/Chendemo12/micromq/sdk"
	"github.com/Chendemo12/micromq/src/proto"
	"strconv"
	"strings"
	"time"
)

const DefaultLoggerTopic string = "MQ_LISTENER"

func New(conf *config.AppConfig) *Monitor {
	m := &Monitor{Conf: conf}

	m.LoggerTopicKey = m.LoggerTopic() + "_" + strings.ToUpper(conf.Name)
	m.chandler = &ConsumerHandler{m: m}
	m.phandler = &ProducerHandler{m: m}

	m.store = make([]*Container, 0, len(conf.Listeners))
	for _, t := range conf.Listeners {
		m.Container(t.Topic)
	}

	return m
}

type Monitor struct {
	logger         logger.Iface
	chandler       *ConsumerHandler
	phandler       *ProducerHandler
	consumer       *sdk.Consumer
	producer       *sdk.Producer
	store          []*Container
	Conf           *config.AppConfig
	Ctx            context.Context
	Cancel         context.CancelFunc
	LoggerTopicKey string // 系统调用执行结果通知topic-key
}

func (m *Monitor) Logger() logger.Iface { return m.logger }

func (m *Monitor) LoggerConf() *zaplog.Config {
	// logger
	c := &zaplog.Config{
		Filename:   m.LoggerTopicKey,
		Level:      zaplog.WARNING,
		Rotation:   10,
		Retention:  10,
		MaxBackups: 4,
		Compress:   !m.Conf.Debug,
	}
	if m.Conf.Debug {
		c.Level = zaplog.DEBUG
	}

	return c
}

func (m *Monitor) MQConf() sdk.Config {
	return sdk.Config{
		Host:   m.Conf.Broker.Host,
		Port:   m.Conf.Broker.Port,
		Ack:    sdk.AllConfirm,
		Logger: m.Logger(),
		Token:  m.Conf.Broker.Token,
	}
}

func (m *Monitor) LoggerTopic() string {
	if m.Conf.LoggerTopic != "" {
		return m.Conf.LoggerTopic
	}
	return DefaultLoggerTopic
}

// FindListener 查找配置
func (m *Monitor) FindListener(topic string) *config.Listener {
	for _, listener := range m.Conf.Listeners {
		if listener.Topic == topic {
			return listener
		}
	}
	return nil // 肯定存在
}

func (m *Monitor) Container(topic string) *Container {
	for i := 0; i < len(m.store); i++ {
		if m.store[i] != nil && m.store[i].Topic == topic {
			return m.store[i]
		}
	}

	t := m.FindListener(topic)
	cont := &Container{
		Topic:          topic,
		Listener:       t,
		ValueCallField: t.Monitor.FieldName,
		Cmd:            strings.Split(t.Monitor.Cmd, " "),
		LastKey:        "",
		CurrentKey:     "",
		LastValueCS:    0,
		CurrentValueCS: 0,
		LastValue:      t.NewValueInstance(),
		CurrentValue:   t.NewValueInstance(),
		UpdateTime:     time.Now(),
		Unmarshal:      func(b []byte, v any) error { return nil }, // do nothing
	}

	// 需要反序列化
	if t.NeedUnmarshalValue() {
		cont.Unmarshal = helper.JsonUnmarshal
	}

	m.store = append(m.store, cont)
	return cont
}

// NewValueInstance 创建指定topic的值副本
func (m *Monitor) NewValueInstance(topic string) map[string]any {
	if listener := m.Container(topic).Listener; listener != nil {
		return listener.NewValueInstance()
	}
	return map[string]any{} // 构造一个空字典
}

func (m *Monitor) CallByKey(topic string) bool {
	if listener := m.Container(topic).Listener; listener != nil {
		return listener.CallByKey()
	}
	return false
}

func (m *Monitor) CallByValue(topic string) bool {
	if listener := m.Container(topic).Listener; listener != nil {
		return listener.CallByValue()
	}
	return false
}

// ValueCallField 触发系统调用的值字段
func (m *Monitor) ValueCallField(topic string) string {
	if listener := m.Container(topic).Listener; listener != nil {
		if listener.CallByValue() {
			return listener.Monitor.FieldName
		}
	}
	return ""
}

// CallOnReceived 是否在收到消息时触发系统调用
func (m *Monitor) CallOnReceived(topic string) bool {
	if listener := m.Container(topic).Listener; listener != nil {
		return listener.Monitor.OnEvent == config.CallOnReceived
	}
	return false
}

// CallOnUpdated 是否在数据更新时触发系统调用
func (m *Monitor) CallOnUpdated(topic string) bool {
	if listener := m.Container(topic).Listener; listener != nil {
		return listener.Monitor.OnEvent == config.CallOnUpdated
	}
	return false
}

// UpdateCMRecord 更新消息记录
func (m *Monitor) UpdateCMRecord(message *sdk.ConsumerMessage) error {
	cont := m.Container(message.Topic).exchange()

	cont.CurrentKey = message.Key
	cont.CurrentValueCS = proto.CalcChecksum(message.Value)
	form := m.NewValueInstance(message.Topic)

	// 解析消息体
	err := cont.Unmarshal(message.Value, &form)
	if err == nil {
		for k, v := range form {
			cont.CurrentValue[k] = v
		}
	}

	return err
}

// IsKeyUpdated 判断KEY是否发生变化，此方法应在 UpdateRecord 更新记录之后调用
// 且应首先判断调用条件是否是依据 TOPIC-KEY，调用时机是否是 OnUpdated
//
//	Usage:
//
//	if m.CallOnReceived(topic) {
//		m.UpdateCMRecord(message) // 可选
//		sys call
//	}
//
//	if m.CallOnUpdated(topic) {
//		m.UpdateCMRecord(message)
//		if m.CallByKey(topic) && m.IsKeyUpdated(topic) {
//			sys call
//		}
//		if m.CallByValue(topic) && m.IsValueUpdated(topic) {
//			sys call
//		}
//	}
func (m *Monitor) IsKeyUpdated(topic string) bool {
	cont := m.Container(topic)
	return cont.LastKey != cont.CurrentKey
}

// IsValueUpdated 判断Value是否发生变化，此方法应在 UpdateRecord 更新记录之后调用
// 且应首先判断调用条件是否是依据 TOPIC-VALUE，调用时机是否是 OnUpdated
func (m *Monitor) IsValueUpdated(topic string) bool {
	cont := m.Container(topic)
	// 不需要反序列化
	if !cont.Listener.NeedUnmarshalValue() {
		return cont.LastValueCS != cont.CurrentValueCS
	} else {
		return cont.CurrentValue[cont.ValueCallField] != cont.LastValue[cont.ValueCallField]
	}
}

// MakeCmd 构造系统调用命令, 将指定的KEY或VALUE的字段值作为参数传递
// 对于 monitor.Where == key: 将当前的key作为参数传递
// 对于 monitor.Where == value: 将当前的 value.fieldName 的值作为参数传递，若fieldName未指定，则为空
func (m *Monitor) MakeCmd(topic string, by config.CallByWhat) []string {
	cont := m.Container(topic)

	if by == config.CallByKey {
		return append(cont.Cmd, cont.CurrentKey)
	}

	// call by value
	args := ""
	v := cont.CurrentValue[cont.ValueCallField]

	// 依据 config.FieldType 类型进行推断
	switch v.(type) {
	case string:
		args = v.(string)
	case bool:
		args = strconv.FormatBool(v.(bool))
	case float32: // 默认设置为 float32
		args = strconv.FormatFloat(float64(v.(float32)), 'f', -1, 64)
	case float64:
		args = strconv.FormatFloat(v.(float64), 'f', -1, 64)
	case int:
		args = strconv.Itoa(v.(int))
	default: // map 类型
		b, err := helper.JsonMarshal(v)
		if err == nil {
			args = string(b)
		}
	}

	return append(cont.Cmd, args)
}

// Publisher 将topic事件执行结果发送到mq
func (m *Monitor) Publisher(form *LoggerReportForm) {
	err := m.producer.Send(func(record *sdk.ProducerMessage) error {
		record.Topic = m.LoggerTopic()
		record.Key = m.LoggerTopicKey

		return record.BindFromJSON(form)
	})

	if err != nil {
		m.logger.Error("report publisher failed: ", err.Error())
	} else {
		m.logger.Info("report publisher.")
	}
}

func (m *Monitor) Start() {
	m.logger = logger.NewDefaultLogger()
	//m.logger = zaplog.NewLogger(m.LoggerConf()).Sugar()

	con, err := sdk.NewConsumer(m.MQConf(), m.chandler)
	if m.Conf.Broker.MessageEncrypt {
		con.SetCryptoPlan(m.Conf.Broker.MessageEncryptPlan)
	}
	err = con.Start()
	if err != nil {
		panic(err)
	}

	prod := sdk.NewProducer(m.MQConf(), m.phandler)
	if m.Conf.Broker.MessageEncrypt {
		prod.SetCryptoPlan(m.Conf.Broker.MessageEncryptPlan)
	}
	err = prod.Start()
	if err != nil {
		panic(err)
	}

	m.consumer = con
	m.producer = prod
}

func (m *Monitor) Stop() {
	if m.consumer != nil && m.consumer.IsConnected() {
	}
}

type Container struct {
	Topic          string
	Listener       *config.Listener
	ValueCallField string
	Cmd            []string
	LastKey        string
	CurrentKey     string
	LastValueCS    uint16 // 如果value未定义反序列化方法，则判断原始字节序列的校验码
	CurrentValueCS uint16
	LastValue      map[string]any // 用于判断值是否发生变化
	CurrentValue   map[string]any
	UpdateTime     time.Time
	Unmarshal      func(b []byte, v any) error // value 反序列化方法
}

// 交换记录
func (c *Container) exchange() *Container {
	c.LastValueCS = c.CurrentValueCS
	c.LastKey = c.CurrentKey
	for k, v := range c.CurrentValue {
		c.LastValue[k] = v
	}

	c.UpdateTime = time.Now()

	return c
}

func (c *Container) KeyString() string {
	return c.LastKey + " -> " + c.CurrentKey
}

func (c *Container) ValueString() string {
	if c.ValueCallField != "" {
		return fmt.Sprintf(
			"%s -> %s",
			c.LastValue[c.ValueCallField], c.CurrentValue[c.ValueCallField],
		)
	}
	return fmt.Sprintf("%d -> %d", c.LastValueCS, c.CurrentValueCS)
}
