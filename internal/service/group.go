package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/model"
	"go-chat/internal/repository/repo"
	"gorm.io/gorm"

	"go-chat/internal/entity"
	"go-chat/internal/pkg/jsonutil"
	"go-chat/internal/pkg/sliceutil"
	"go-chat/internal/pkg/timeutil"
)

type GroupService struct {
	*BaseService
	repo      *repo.Group
	memberDao *repo.GroupMember
	relation  *cache.Relation
}

func NewGroupService(baseService *BaseService, repo *repo.Group, memberDao *repo.GroupMember, relation *cache.Relation) *GroupService {
	return &GroupService{BaseService: baseService, repo: repo, memberDao: memberDao, relation: relation}
}

func (s *GroupService) Dao() *repo.Group {
	return s.repo
}

type CreateGroupOpt struct {
	UserId    int    // 操作人ID
	Name      string // 群名称
	Avatar    string // 群头像
	Profile   string // 群简介
	MemberIds []int  // 联系人ID
}

// Create 创建群聊
func (s *GroupService) Create(ctx context.Context, opts *CreateGroupOpt) (int, error) {
	var (
		err      error
		members  []*model.GroupMember
		talkList []*model.TalkSession
	)

	// 群成员用户ID
	mids := sliceutil.Unique(append(opts.MemberIds, opts.UserId))

	group := &model.Group{
		CreatorId: opts.UserId,
		Name:      opts.Name,
		Profile:   opts.Profile,
		Avatar:    opts.Avatar,
		MaxNum:    model.GroupMemberMaxNum,
	}

	joinTime := time.Now()

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err = tx.Create(group).Error; err != nil {
			return err
		}

		for _, val := range mids {
			leader := 0
			if opts.UserId == val {
				leader = 2
			}

			members = append(members, &model.GroupMember{
				GroupId:  group.Id,
				UserId:   val,
				Leader:   leader,
				JoinTime: joinTime,
			})

			talkList = append(talkList, &model.TalkSession{
				TalkType:   2,
				UserId:     val,
				ReceiverId: group.Id,
			})
		}

		if err = tx.Create(members).Error; err != nil {
			return err
		}

		if err = tx.Create(talkList).Error; err != nil {
			return err
		}

		record := &model.TalkRecords{
			TalkType:   entity.ChatGroupMode,
			ReceiverId: group.Id,
			MsgType:    entity.MsgTypeGroupInvite,
		}
		if err = tx.Create(record).Error; err != nil {
			return err
		}

		if err = tx.Create(&model.TalkRecordsInvite{
			RecordId:      record.Id,
			Type:          1,
			OperateUserId: opts.UserId,
			UserIds:       sliceutil.ToIds(mids[0 : len(mids)-1]),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	// 广播网关将在线的用户加入房间
	body := map[string]interface{}{
		"event": entity.EventTalkJoinGroup,
		"data": jsonutil.Encode(map[string]interface{}{
			"group_id": group.Id,
			"uids":     mids,
		}),
	}

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(body))

	return group.Id, err
}

type UpdateGroupOpt struct {
	GroupId int    // 群ID
	Name    string // 群名称
	Avatar  string // 群头像
	Profile string // 群简介
}

// Update 更新群信息
func (s *GroupService) Update(ctx context.Context, opts *UpdateGroupOpt) error {

	_, err := s.repo.UpdateById(ctx, opts.GroupId, map[string]interface{}{
		"group_name": opts.Name,
		"avatar":     opts.Avatar,
		"profile":    opts.Profile,
	})

	return err
}

// Dismiss 解散群组[群主权限]
func (s *GroupService) Dismiss(_ context.Context, groupId int, uid int) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Group{Id: groupId, CreatorId: uid}).Updates(&model.Group{
			IsDismiss: 1,
		}).Error; err != nil {
			return err
		}

		if err := s.db.Model(&model.GroupMember{}).Where("group_id = ?", groupId).Updates(&model.GroupMember{
			IsQuit: 1,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	return err
}

// Secede 退出群组[仅管理员及群成员]
func (s *GroupService) Secede(ctx context.Context, groupId int, uid int) error {

	info := &model.GroupMember{}
	if err := s.db.Where("group_id = ? AND user_id = ? and is_quit = 0", groupId, uid).First(info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("数据不存在！")
		}

		return err
	}

	if info.Leader == 2 {
		return errors.New("群主不能退出群组！")
	}

	record := &model.TalkRecords{
		TalkType:   entity.ChatGroupMode,
		ReceiverId: groupId,
		MsgType:    entity.MsgTypeGroupInvite,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, uid).Updates(&model.GroupMember{
			IsQuit: 1,
		}).Error
		if err != nil {
			return err
		}

		if err = tx.Create(record).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.TalkRecordsInvite{
			RecordId:      record.Id,
			Type:          2,
			OperateUserId: uid,
			UserIds:       fmt.Sprintf("%v", uid),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.relation.DelGroupRelation(ctx, uid, groupId)

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalkJoinGroup,
		"data": jsonutil.Encode(map[string]interface{}{
			"type":     2,
			"group_id": groupId,
			"uids":     []int{uid},
		}),
	}))

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalk,
		"data": jsonutil.Encode(map[string]interface{}{
			"sender_id":   record.UserId,
			"receiver_id": record.ReceiverId,
			"talk_type":   record.TalkType,
			"record_id":   record.Id,
		}),
	}))

	return nil
}

type InviteGroupMembersOpt struct {
	UserId    int   // 操作人ID
	GroupId   int   // 群ID
	MemberIds []int // 群成员ID
}

// InviteMembers 邀请加入群聊
func (s *GroupService) InviteMembers(ctx context.Context, opts *InviteGroupMembersOpt) error {
	var (
		err            error
		addMembers     []*model.GroupMember
		addTalkList    []*model.TalkSession
		updateTalkList []int
		talkList       []*model.TalkSession
	)

	m := make(map[int]struct{})
	for _, value := range s.memberDao.GetMemberIds(ctx, opts.GroupId) {
		m[value] = struct{}{}
	}

	listHash := make(map[int]*model.TalkSession)
	s.db.Select("id", "user_id", "is_delete").Where("user_id in ? and receiver_id = ? and talk_type = 2", opts.MemberIds, opts.GroupId).Find(&talkList)
	for _, item := range talkList {
		listHash[item.UserId] = item
	}

	for _, value := range opts.MemberIds {
		if _, ok := m[value]; !ok {
			addMembers = append(addMembers, &model.GroupMember{
				GroupId:  opts.GroupId,
				UserId:   value,
				JoinTime: time.Now(),
			})
		}

		if item, ok := listHash[value]; !ok {
			addTalkList = append(addTalkList, &model.TalkSession{
				TalkType:   entity.ChatGroupMode,
				UserId:     value,
				ReceiverId: opts.GroupId,
			})
		} else if item.IsDelete == 1 {
			updateTalkList = append(updateTalkList, item.Id)
		}
	}

	if len(addMembers) == 0 {
		return errors.New("邀请的好友，都已成为群成员")
	}

	record := &model.TalkRecords{
		TalkType:   entity.ChatGroupMode,
		ReceiverId: opts.GroupId,
		MsgType:    entity.MsgTypeGroupInvite,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 删除已存在成员记录
		tx.Where("group_id = ? and user_id in ? and is_quit = 1", opts.GroupId, opts.MemberIds).Delete(&model.GroupMember{})

		// 添加新成员
		if err = tx.Create(&addMembers).Error; err != nil {
			return err
		}

		// 添加用户的对话列表
		if len(addTalkList) > 0 {
			if err = tx.Select("talk_type", "user_id", "receiver_id", "updated_at").Create(&addTalkList).Error; err != nil {
				return err
			}
		}

		// 更新用户的对话列表
		if len(updateTalkList) > 0 {
			tx.Model(&model.TalkSession{}).Where("id in ?", updateTalkList).Updates(map[string]interface{}{
				"is_delete":  0,
				"created_at": timeutil.DateTime(),
			})
		}

		if err = tx.Create(record).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.TalkRecordsInvite{
			RecordId:      record.Id,
			Type:          1,
			OperateUserId: opts.UserId,
			UserIds:       sliceutil.ToIds(opts.MemberIds),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 广播网关将在线的用户加入房间
	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalkJoinGroup,
		"data": jsonutil.Encode(map[string]interface{}{
			"type":     1,
			"group_id": opts.GroupId,
			"uids":     opts.MemberIds,
		}),
	}))

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalk,
		"data": jsonutil.Encode(map[string]interface{}{
			"sender_id":   record.UserId,
			"receiver_id": record.ReceiverId,
			"talk_type":   record.TalkType,
			"record_id":   record.Id,
		}),
	}))

	return nil
}

type RemoveMembersOpt struct {
	UserId    int   // 操作人ID
	GroupId   int   // 群ID
	MemberIds []int // 群成员ID
}

// RemoveMembers 群成员移除群聊
func (s *GroupService) RemoveMembers(ctx context.Context, opts *RemoveMembersOpt) error {
	var num int64

	if err := s.Db().Model(&model.GroupMember{}).Where("group_id = ? and user_id in ? and is_quit = 0", opts.GroupId, opts.MemberIds).Count(&num).Error; err != nil {
		return err
	}

	if int(num) != len(opts.MemberIds) {
		return errors.New("删除失败")
	}

	record := &model.TalkRecords{
		TalkType:   entity.ChatGroupMode,
		ReceiverId: opts.GroupId,
		MsgType:    entity.MsgTypeGroupInvite,
	}

	err := s.Db().Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&model.GroupMember{}).Where("group_id = ? and user_id in ? and is_quit = 0", opts.GroupId, opts.MemberIds).Updates(map[string]interface{}{
			"is_quit":    1,
			"deleted_at": time.Now(),
		}).Error
		if err != nil {
			return err
		}

		if err = tx.Create(record).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.TalkRecordsInvite{
			RecordId:      record.Id,
			Type:          3,
			OperateUserId: opts.UserId,
			UserIds:       sliceutil.ToIds(opts.MemberIds),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	// 推送消息
	if err != nil {
		return err
	}

	s.relation.BatchDelGroupRelation(ctx, opts.MemberIds, opts.GroupId)

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalkJoinGroup,
		"data": jsonutil.Encode(map[string]interface{}{
			"type":     2,
			"group_id": opts.GroupId,
			"uids":     opts.MemberIds,
		}),
	}))

	s.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalk,
		"data": jsonutil.Encode(map[string]interface{}{
			"sender_id":   int64(record.UserId),
			"receiver_id": int64(record.ReceiverId),
			"talk_type":   record.TalkType,
			"record_id":   int64(record.Id),
		}),
	}))

	return nil
}

type session struct {
	ReceiverID int `json:"receiver_id"`
	IsDisturb  int `json:"is_disturb"`
}

func (s *GroupService) List(userId int) ([]*model.GroupItem, error) {
	tx := s.db.Table("group_member")
	tx.Select("`group`.id,`group`.group_name,`group`.avatar,`group`.profile,group_member.leader")
	tx.Joins("left join `group` on `group`.id = group_member.group_id")
	tx.Where("group_member.user_id = ? and group_member.is_quit = ?", userId, 0)
	tx.Order("group_member.created_at desc")

	items := make([]*model.GroupItem, 0)
	if err := tx.Scan(&items).Error; err != nil {
		return nil, err
	}

	length := len(items)
	if length == 0 {
		return items, nil
	}

	ids := make([]int, 0, length)
	for i := range items {
		ids = append(ids, items[i].Id)
	}

	query := s.db.Table("talk_session")
	query.Select("receiver_id,is_disturb")
	query.Where("talk_type = ? and receiver_id in ?", 2, ids)

	list := make([]*session, 0)
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}

	hash := make(map[int]*session)
	for i := range list {
		hash[list[i].ReceiverID] = list[i]
	}

	for i := range items {
		if value, ok := hash[items[i].Id]; ok {
			items[i].IsDisturb = value.IsDisturb
		}
	}

	return items, nil
}
