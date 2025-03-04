package repo

import (
	"context"

	"go-chat/internal/pkg/ichat"
	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type GroupMember struct {
	ichat.Repo[model.GroupMember]
	relation *cache.Relation
}

func NewGroupMember(db *gorm.DB, relation *cache.Relation) *GroupMember {
	return &GroupMember{Repo: ichat.NewRepo[model.GroupMember](db), relation: relation}
}

// IsMaster 判断是否是群主
func (g *GroupMember) IsMaster(ctx context.Context, gid, uid int) bool {

	exist, err := g.QueryExist(ctx, "group_id = ? and user_id = ? and leader = 2 and is_quit = 0", gid, uid)
	if err != nil {
		return false
	}

	return exist
}

// IsLeader 判断是否是群主或管理员
func (g *GroupMember) IsLeader(ctx context.Context, gid, uid int) bool {

	exist, err := g.QueryExist(ctx, "group_id = ? and user_id = ? and leader in (1,2) and is_quit = 0", gid, uid)
	if err != nil {
		return false
	}

	return exist
}

// IsMember 检测是属于群成员
func (g *GroupMember) IsMember(ctx context.Context, gid, uid int, cache bool) bool {

	if cache && g.relation.IsGroupRelation(ctx, uid, gid) == nil {
		return true
	}

	exist, err := g.QueryExist(ctx, "group_id = ? and user_id = ? and is_quit = 0", gid, uid)
	if err != nil {
		return false
	}

	if exist {
		g.relation.SetGroupRelation(ctx, uid, gid)
	}

	return exist
}

// GetMemberIds 获取所有群成员用户ID
func (g *GroupMember) GetMemberIds(ctx context.Context, groupId int) []int {

	var ids []int
	_ = g.Model(ctx).Select("user_id").Where("group_id = ? and is_quit = ?", groupId, 0).Scan(&ids)

	return ids
}

// GetUserGroupIds 获取所有群成员ID
func (g *GroupMember) GetUserGroupIds(ctx context.Context, uid int) []int {

	var ids []int
	_ = g.Model(ctx).Where("user_id = ? and is_quit = ?", uid, 0).Pluck("group_id", &ids)

	return ids
}

// CountMemberTotal 统计群成员总数
func (g *GroupMember) CountMemberTotal(ctx context.Context, gid int) int64 {
	count, _ := g.QueryCount(ctx, "group_id = ? and is_quit = 0", gid)
	return count
}

// GetMemberRemark 获取指定群成员的备注信息
func (g *GroupMember) GetMemberRemark(ctx context.Context, groupId int, userId int) string {

	var remarks string
	g.Model(ctx).Select("user_card").Where("group_id = ? and user_id = ?", groupId, userId).Scan(&remarks)

	return remarks
}

// GetMembers 获取群组成员列表
func (g *GroupMember) GetMembers(ctx context.Context, groupId int) []*model.MemberItem {
	fields := []string{
		"group_member.id",
		"group_member.leader",
		"group_member.user_card",
		"group_member.user_id",
		"group_member.is_mute",
		"users.avatar",
		"users.nickname",
		"users.gender",
		"users.motto",
	}

	tx := g.Db.WithContext(ctx).Table("group_member")
	tx.Joins("left join users on users.id = group_member.user_id")
	tx.Where("group_member.group_id = ? and group_member.is_quit = ?", groupId, 0)
	tx.Order("group_member.leader desc")

	items := make([]*model.MemberItem, 0)
	tx.Unscoped().Select(fields).Scan(&items)

	return items
}

type CountGroupMember struct {
	GroupId int `gorm:"column:group_id;"`
	Count   int `gorm:"column:count;"`
}

func (g *GroupMember) CountGroupMemberNum(ids []int) ([]*CountGroupMember, error) {

	var items []*CountGroupMember
	err := g.Model(context.Background()).Select("group_id,count(*) as count").Where("group_id in ? and is_quit = 0", ids).Group("group_id").Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (g *GroupMember) CheckUserGroup(ids []int, userId int) ([]int, error) {
	items := make([]int, 0)

	err := g.Model(context.Background()).Select("group_id").Where("group_id in ? and user_id = ? and is_quit = 0", ids, userId).Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}
