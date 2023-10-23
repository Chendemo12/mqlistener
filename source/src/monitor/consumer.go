package monitor

import (
	"fmt"
	"gitee.com/chendemo12/mqlistener/src/config"
	"github.com/Chendemo12/fastapi-tool/helper"
	"github.com/Chendemo12/micromq/sdk"
	"os/exec"
	"strings"
)

type ConsumerHandler struct {
	sdk.CHandler
	m *Monitor
}

func (c ConsumerHandler) Topics() []string {
	topics := make([]string, 0, len(c.m.Conf.Topics()))
	for _, k := range c.m.Conf.Topics() {
		if k != c.m.LoggerTopic() { // 避免循环调用，不允许监听发送topic
			topics = append(topics, k)
		}
	}

	return topics
}

func (c ConsumerHandler) OnConnected() {
	c.m.Logger().Info("consumer connected.")
}

func (c ConsumerHandler) OnClosed() {
	c.m.Logger().Warn("consumer connection lost, reconnect..")
}

func (c ConsumerHandler) sysCall(listener *config.Listener) {
	// 对于同一个 topic 的系统调用应同步进行
	listener.Monitor.Lock()
	defer listener.Monitor.UnLock()

	cmd := c.m.MakeCmd(listener.Topic, listener.Monitor.Where)
	form := &LoggerReportForm{
		ListenTopic: listener.Topic,
		OnEvent:     listener.Monitor.OnEvent,
		Where:       listener.Monitor.Where,
		FieldName:   listener.Monitor.FieldName,
		Cmd:         strings.Join(cmd, " "),
		Result:      "",
		Output:      "",
	}

	c.m.Logger().Info(fmt.Sprintf(
		"Topic::%s, syscall:: '%s'", listener.Topic, form.Cmd,
	))

	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	form.Output = helper.B2S(out)

	if err != nil {
		form.Result = CmdFailed
		c.m.Logger().Warn("monitor call failed: ", form.Output)
	} else {
		form.Result = CmdSucceed
		c.m.Logger().Debug("monitor call succeed, output: ", form.Output)
	}

	// 发生执行结果事件
	c.m.Publisher(form)
}

func (c ConsumerHandler) Handler(record *sdk.ConsumerMessage) {
	// 不可异步处理，record 会被释放
	err := c.m.UpdateCMRecord(record) // 都更新一下好了
	callByValue := c.m.CallByValue(record.Topic)

	if err != nil {
		c.m.Logger().Error("parse value failed: ", err.Error())
		if callByValue {
			return
		}
	}

	cont := c.m.Container(record.Topic)
	c.m.Logger().Info(fmt.Sprintf(
		"call on %s by %s | %s",
		cont.Listener.Monitor.OnEvent, cont.Listener.Monitor.Where, record,
	))

	if c.m.CallOnReceived(record.Topic) {
		go c.sysCall(cont.Listener)

	} else if c.m.CallOnUpdated(record.Topic) { // 不应else，后期可能增加其他类型

		if c.m.CallByKey(record.Topic) && c.m.IsKeyUpdated(record.Topic) {
			c.m.Logger().Info("call by key, ", cont.KeyString())

			go c.sysCall(cont.Listener)
			return
		}

		if callByValue && c.m.IsValueUpdated(record.Topic) {
			c.m.Logger().Info("call by value::", cont.ValueCallField, ", ", cont.ValueString())

			go c.sysCall(cont.Listener)
			return
		}
	}
}
