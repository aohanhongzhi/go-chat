package chat

import (
	"context"
	"encoding/json"

	"go-chat/internal/entity"
	"go-chat/internal/pkg/im"
	"go-chat/internal/pkg/jsonutil"
)

type KeyboardMessage struct {
	Event string `json:"event"`
	Data  struct {
		SenderID   int `json:"sender_id"`
		ReceiverID int `json:"receiver_id"`
	} `json:"data"`
}

// OnKeyboard 键盘输入事件
func (h *Handler) OnKeyboard(ctx context.Context, _ im.IClient, data []byte) {

	var m *KeyboardMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}

	h.redis.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(entity.MapStrAny{
		"event": entity.EventTalkKeyboard,
		"data": jsonutil.Encode(entity.MapStrAny{
			"sender_id":   m.Data.SenderID,
			"receiver_id": m.Data.ReceiverID,
		}),
	}))
}
