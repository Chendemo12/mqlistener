package monitor

import (
	"gitee.com/chendemo12/mqlistener/src/config"
	"github.com/Chendemo12/micromq/sdk"
)

type CmdResult string

const (
	CmdFailed  CmdResult = "FAILED"
	CmdSucceed CmdResult = "SUCCEED"
)

type LoggerReportForm struct {
	ListenTopic string             `json:"listen_topic" yaml:"ListenTopic" description:"触发topic"`
	OnEvent     config.CallOnEvent `json:"on_event" yaml:"onEvent" description:"触发时机"`
	Where       config.CallByWhat  `json:"where" yaml:"where" description:"触发依据"`
	FieldName   string             `json:"field_name" yaml:"fieldName" description:"事件触发字段"`
	Cmd         string             `json:"cmd" yaml:"cmd" description:"系统调用命令"`
	Result      CmdResult          `json:"result" yaml:"result" description:"系统调用执行结果"`
	Output      string             `json:"output" yaml:"output" description:"系统调用执行输出"`
}

type ProducerHandler struct {
	sdk.PHandler
	m *Monitor
}
