package service

import (
	"context"
	"errors"

	"go-chat/internal/repository/model"
	"go-chat/internal/repository/repo"
)

type GroupApplyService struct {
	*BaseService
	repo *repo.GroupApply
}

func NewGroupApplyService(baseService *BaseService, repo *repo.GroupApply) *GroupApplyService {
	return &GroupApplyService{BaseService: baseService, repo: repo}
}

func (s *GroupApplyService) Dao() *repo.GroupApply {
	return s.repo
}

func (s *GroupApplyService) Auth(ctx context.Context, applyId, userId int) bool {
	info := &model.GroupApply{}

	err := s.Db().First(info, "id = ?", applyId).Error
	if err != nil {
		return false
	}

	member := &model.GroupMember{}
	err = s.Db().First(member, "group_id = ? and user_id = ? and leader in (1,2) and is_quit = 0", info.GroupId).Error
	if err != nil {
		return false
	}

	return member.Id == 0
}

func (s *GroupApplyService) Insert(ctx context.Context, groupId, userId int, remark string) error {
	return s.Db().Create(&model.GroupApply{
		GroupId: groupId,
		UserId:  userId,
		Remark:  remark,
	}).Error
}

func (s *GroupApplyService) Delete(ctx context.Context, applyId, userId int) error {

	if !s.Auth(ctx, applyId, userId) {
		return errors.New("auth failed")
	}

	return s.Db().Delete(&model.GroupApply{}, "id = ?", applyId).Error
}
