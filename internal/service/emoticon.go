package service

import (
	"fmt"
	"strconv"
	"strings"

	"go-chat/internal/pkg/filesystem"
	"go-chat/internal/pkg/sliceutil"
	"go-chat/internal/repository/model"
	"go-chat/internal/repository/repo"
)

type EmoticonService struct {
	*BaseService
	repo       *repo.Emoticon
	fileSystem *filesystem.Filesystem
}

func NewEmoticonService(baseService *BaseService, repo *repo.Emoticon, fileSystem *filesystem.Filesystem) *EmoticonService {
	return &EmoticonService{BaseService: baseService, repo: repo, fileSystem: fileSystem}
}

func (s *EmoticonService) Dao() *repo.Emoticon {
	return s.repo
}

func (s *EmoticonService) RemoveUserSysEmoticon(uid int, emoticonId int) error {
	ids := s.repo.GetUserInstallIds(uid)

	if !sliceutil.Include(emoticonId, ids) {
		return fmt.Errorf("数据不存在！")
	}

	items := make([]string, 0, len(ids)-1)

	for _, id := range ids {
		if id != emoticonId {
			items = append(items, strconv.Itoa(id))
		}
	}

	return s.db.Model(&model.UsersEmoticon{}).Where("user_id = ?", uid).Update("emoticon_ids", strings.Join(items, ",")).Error
}

func (s *EmoticonService) AddUserSysEmoticon(uid int, emoticonId int) error {
	ids := s.repo.GetUserInstallIds(uid)

	if sliceutil.Include(emoticonId, ids) {
		return nil
	}

	ids = append(ids, emoticonId)

	return s.db.Model(&model.UsersEmoticon{}).Where("user_id = ?", uid).Update("emoticon_ids", sliceutil.ToIds(ids)).Error
}

// DeleteCollect 删除自定义表情包
func (s *EmoticonService) DeleteCollect(uid int, ids []int) error {
	return s.db.Delete(&model.EmoticonItem{}, "id in ? and emoticon_id = 0 and user_id = ?", ids, uid).Error
}
