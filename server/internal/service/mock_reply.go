package service

import (
	"fmt"
	"strings"

	"github.com/agentzero/server/internal/model"
)

// MockReply 第一阶段：智能体的真实对话能力还没接 LLM，
// 这里用一段「在线但风格化」的占位回复，让 UI 跑通且看起来不假。
func MockReply(agent *model.Agent, userInput string) string {
	if agent == nil {
		return "（还没有连上模型，先用占位回复回应一下你说的：" + userInput + "）"
	}
	caps := ""
	if len(agent.Capabilities) > 0 {
		caps = "我擅长：" + strings.Join(agent.Capabilities, "、") + "。\n\n"
	}
	return fmt.Sprintf(
		"我是 %s，你的「%s」。\n\n%s你刚才说：「%s」\n\n这是一段示例回复——这个版本里我的真实大脑还在路上，等服务端接好模型，我会用真本事回答你。",
		agent.Name, agent.Tagline, caps, userInput,
	)
}
