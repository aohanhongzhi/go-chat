package repo

import (
	"context"
	"fmt"

	"go-chat/internal/pkg/ichat"
	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type Contact struct {
	ichat.Repo[model.Contact]
	cache    *cache.ContactRemark
	relation *cache.Relation
}

func NewContact(db *gorm.DB, cache *cache.ContactRemark, relation *cache.Relation) *Contact {
	return &Contact{Repo: ichat.NewRepo[model.Contact](db), cache: cache, relation: relation}
}

func (c *Contact) Remarks(ctx context.Context, uid int, fids []int) (map[int]string, error) {

	if !c.cache.IsExist(ctx, uid) {
		_ = c.LoadContactCache(ctx, uid)
	}

	return c.cache.MGet(ctx, uid, fids)
}

// IsFriend 判断是否为好友关系
func (c *Contact) IsFriend(ctx context.Context, uid int, friendId int, cache bool) bool {

	if cache && c.relation.IsContactRelation(ctx, uid, friendId) == nil {
		return true
	}

	count, err := c.QueryCount(ctx, "((user_id = ? and friend_id = ?) or (user_id = ? and friend_id = ?)) and status = 1", uid, friendId, friendId, uid)
	if err != nil {
		return false
	}

	if count == 2 {
		c.relation.SetContactRelation(ctx, uid, friendId)
	} else {
		c.relation.DelContactRelation(ctx, uid, friendId)
	}

	return count == 2
}

func (c *Contact) GetFriendRemark(ctx context.Context, uid int, friendId int) string {

	if c.cache.IsExist(ctx, uid) {
		return c.cache.Get(ctx, uid, friendId)
	}

	info, err := c.FindByWhere(ctx, "user_id = ? and friend_id = ?", uid, friendId)
	if err != nil {
		return ""
	}

	return info.Remark
}

func (c *Contact) SetFriendRemark(ctx context.Context, uid int, friendId int, remark string) error {
	return c.cache.Set(ctx, uid, friendId, remark)
}

func (c *Contact) LoadContactCache(ctx context.Context, uid int) error {

	contacts, err := c.FindAll(ctx, func(db *gorm.DB) {
		db.Where("user_id = ? and status = 1", uid)
	})

	if err != nil {
		return err
	}

	items := make(map[string]interface{})
	for _, value := range contacts {
		if len(value.Remark) > 0 {
			items[fmt.Sprintf("%d", value.FriendId)] = value.Remark
		}
	}

	_ = c.cache.MSet(ctx, uid, items)

	return nil
}
