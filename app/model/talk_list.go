package model

type TalkList struct {
	ID         int    `json:"id" grom:"comment:聊天列表ID"`
	TalkType   int    `json:"talk_type" grom:"comment:聊天类型"`
	UserId     int    `json:"user_id" grom:"comment:用户ID或消息发送者ID"`
	ReceiverId int    `json:"receiver_id" grom:"comment:接收者ID"`
	IsDelete   int    `json:"is_delete" grom:"comment:是否删除"`
	IsTop      int    `json:"is_top" grom:"comment:是否置顶"`
	IsRobot    int    `json:"is_robot" grom:"comment:消息免打扰"`
	IsDisturb  int    `json:"is_disturb" grom:"comment:是否机器人"`
	CreatedAt  string `json:"created_at" grom:"comment:创建时间"`
	UpdatedAt  string `json:"updated_at" grom:"comment:更新时间"`
}
