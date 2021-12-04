package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/gin-gonic/gin"
	"go-chat/app/dao"
	"go-chat/app/entity"
	"go-chat/app/http/request"
	"go-chat/app/model"
	"go-chat/app/pkg/auth"
	"go-chat/app/pkg/jsonutil"
	"go-chat/app/pkg/slice"
	"gorm.io/gorm"
	"reflect"
	"time"
)

type GroupService struct {
	*BaseService
	dao       *dao.GroupDao
	memberDao *dao.GroupMemberDao
}

func NewGroupService(baseService *BaseService, dao *dao.GroupDao, groupMemberDao *dao.GroupMemberDao) *GroupService {
	return &GroupService{BaseService: baseService, dao: dao, memberDao: groupMemberDao}
}

func (s *GroupService) Dao() *dao.GroupDao {
	return s.dao
}

// Create 创建群聊
func (s *GroupService) Create(ctx *gin.Context, request *request.GroupCreateRequest) error {
	var (
		err      error
		members  []*model.GroupMember
		talkList []*model.TalkList
		groupId  int
	)

	// 登录用户ID
	uid := auth.GetAuthUserID(ctx)

	// 群成员用户ID
	mids := slice.ParseIds(request.MembersIds)
	mids = slice.UniqueInt(append(mids, uid))

	err = s.db.Transaction(func(tx *gorm.DB) error {
		group := &model.Group{
			CreatorId: uid,
			GroupName: request.Name,
			Profile:   request.Profile,
			Avatar:    request.Avatar,
			MaxNum:    model.GroupMemberMaxNum,
			CreatedAt: time.Now(),
		}

		if err = tx.Create(group).Error; err != nil {
			return err
		}

		groupId = group.Id

		for _, val := range mids {
			leader := 0
			if uid == val {
				leader = 2
			}

			members = append(members, &model.GroupMember{
				GroupId:   group.Id,
				UserId:    val,
				Leader:    leader,
				CreatedAt: time.Now(),
			})

			talkList = append(talkList, &model.TalkList{
				TalkType:   2,
				UserId:     val,
				ReceiverId: group.Id,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			})
		}

		if err = tx.Model(&model.GroupMember{}).Create(members).Error; err != nil {
			return err
		}

		if err = tx.Model(&model.TalkList{}).Create(talkList).Error; err != nil {
			return err
		}

		// 需要插入群邀请记录

		return nil
	})

	// 广播网关将在线的用户加入房间
	body := map[string]interface{}{
		"event_name": entity.EventJoinGroupRoom,
		"data": jsonutil.JsonEncode(map[string]interface{}{
			"group_id": groupId,
			"uids":     mids,
		}),
	}

	s.rds.Publish(ctx, entity.SubscribeWsGatewayAll, jsonutil.JsonEncode(body))

	return err
}

// Dismiss 解散群组(群主权限)
func (s *GroupService) Dismiss(GroupId int, UserId int) error {
	var (
		err error
	)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		queryModel := &model.Group{Id: GroupId, CreatorId: UserId}
		dismissedAt := sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		}

		if err = tx.Model(queryModel).Updates(model.Group{IsDismiss: 1, DismissedAt: dismissedAt}).Error; err != nil {
			return err
		}

		if err = s.db.Model(&model.GroupMember{}).Where("group_id = ?", GroupId).Unscoped().Updates(model.GroupMember{
			IsQuit:    1,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		}).Error; err != nil {
			return err
		}

		// 返回 nil 提交事务
		return nil
	})

	return err
}

// Secede 退出群组(仅普通管理员及群成员)
func (s *GroupService) Secede(GroupId int, UserId int) error {
	var err error

	info := &model.GroupMember{}
	err = s.db.Model(model.GroupMember{}).Where("group_id = ? AND user_id = ?", GroupId, UserId).Unscoped().First(info).Error
	if err != nil {
		return err
	}

	if info.Leader == 2 {
		return errors.New("群主不能退出群组！")
	}

	if info.IsQuit == 1 {
		return nil
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		count := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", GroupId, UserId).Unscoped().Updates(model.GroupMember{
			IsQuit:    1,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		}).RowsAffected

		if count == 0 {
			return nil
		}

		// todo 添加群消息

		return nil
	})

	return err
}

// InviteUsers 邀请用户加入群聊
func (s *GroupService) InviteUsers(ctx context.Context, groupId int, uid int, uids []int) error {
	var (
		err            error
		addMembers     []*model.GroupMember
		addTalkList    []*model.TalkList
		updateTalkList []int
		talkList       []*model.TalkList
	)

	m := make(map[int]struct{})
	for _, value := range s.memberDao.GetMemberIds(groupId) {
		m[value] = struct{}{}
	}

	listHash := make(map[int]*model.TalkList)
	s.db.Select("id", "user_id", "is_delete").Where("user_id in ? and receiver_id = ? and talk_type = 2", uids, groupId).Find(&talkList)
	for _, item := range talkList {
		listHash[item.UserId] = item
	}

	for _, value := range uids {
		if _, ok := m[value]; !ok {
			addMembers = append(addMembers, &model.GroupMember{
				GroupId:   groupId,
				UserId:    value,
				CreatedAt: time.Now(),
			})
		}

		if item, ok := listHash[value]; !ok {
			addTalkList = append(addTalkList, &model.TalkList{
				TalkType:   2,
				UserId:     value,
				ReceiverId: groupId,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			})
		} else if item.IsDelete == 1 {
			updateTalkList = append(updateTalkList, item.ID)
		}
	}

	if len(addMembers) == 0 {
		return errors.New("邀请的好友，都已成为群成员")
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 删除已存在成员记录
		tx.Where("group_id = ? and user_id in ? and is_quit = 1", groupId, uids).Unscoped().Delete(model.GroupMember{})

		// 添加新成员
		if err = tx.Omit("deleted_at").Create(&addMembers).Error; err != nil {
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
			tx.Model(&model.TalkList{}).Where("id in ?", updateTalkList).Updates(map[string]interface{}{
				"is_delete":  0,
				"created_at": time.Now(),
			})
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 广播网关将在线的用户加入房间
	body := map[string]interface{}{
		"event_name": entity.EventJoinGroupRoom,
		"data": jsonutil.JsonEncode(map[string]interface{}{
			"group_id": groupId,
			"uids":     uids,
		}),
	}

	s.rds.Publish(ctx, entity.SubscribeWsGatewayAll, jsonutil.JsonEncode(body))

	return nil
}

type Result struct {
	Id        int    `json:"id"`
	GroupName string `json:"group_name"`
	Avatar    string `json:"avatar"`
	Profile   string `json:"profile"`
	Leader    int    `json:"leader"`
	IsDisturb int    `json:"is_disturb"`
}

func (s *GroupService) UserGroupList(userId int) ([]*Result, error) {
	var err error
	items := make([]*Result, 0)

	res := s.db.Table("group_member").
		Select("`group`.id,`group`.group_name,`group`.avatar,`group`.profile,group_member.leader").
		Joins("left join `group` on `group`.id = group_member.group_id").
		Where("group_member.user_id = ? and group_member.is_quit = ?", userId, 0).
		Unscoped().
		Scan(&items)

	if res.Error != nil {
		return nil, res.Error
	}

	if res.RowsAffected == 0 {
		return items, nil
	}

	ids := make([]int, res.RowsAffected)
	for _, item := range items {
		ids = append(ids, item.Id)
	}

	var list []map[string]interface{}
	err = s.db.Table("talk_list").
		Select("receiver_id,is_disturb").
		Where("talk_type = ? and receiver_id in ?", 2, ids).Find(&list).Error
	if err != nil {
		return nil, err
	}

	lists, err := slice.ToMap(list, "receiver_id")
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if data, ok := lists[int64(item.Id)]; ok {
			val := data["is_disturb"]
			item.IsDisturb = int(reflect.ValueOf(val).Int())
		}
	}

	return items, nil
}
