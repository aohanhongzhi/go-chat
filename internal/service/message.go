package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"

	"go-chat/api/pb/message/v1"
	"go-chat/internal/entity"
	"go-chat/internal/logic"
	"go-chat/internal/pkg/encrypt"
	"go-chat/internal/pkg/filesystem"
	"go-chat/internal/pkg/jsonutil"
	"go-chat/internal/pkg/logger"
	"go-chat/internal/pkg/strutil"
	"go-chat/internal/pkg/timeutil"
	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/model"
	"go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

type MessageService struct {
	*BaseService
	forward         *logic.MessageForwardLogic
	groupMemberRepo *repo.GroupMember
	splitUploadRepo *repo.SplitUpload
	fileSystem      *filesystem.Filesystem
	unreadStorage   *cache.UnreadStorage
	messageStorage  *cache.MessageStorage
	sidStorage      *cache.ServerStorage
	clientStorage   *cache.ClientStorage
	Sequence        *repo.Sequence
}

func NewMessageService(baseService *BaseService, forward *logic.MessageForwardLogic, groupMemberRepo *repo.GroupMember, splitUploadRepo *repo.SplitUpload, fileSystem *filesystem.Filesystem, unreadStorage *cache.UnreadStorage, messageStorage *cache.MessageStorage, sidStorage *cache.ServerStorage, clientStorage *cache.ClientStorage, sequence *repo.Sequence) *MessageService {
	return &MessageService{BaseService: baseService, forward: forward, groupMemberRepo: groupMemberRepo, splitUploadRepo: splitUploadRepo, fileSystem: fileSystem, unreadStorage: unreadStorage, messageStorage: messageStorage, sidStorage: sidStorage, clientStorage: clientStorage, Sequence: sequence}
}

// SendText 文本消息
func (m *MessageService) SendText(ctx context.Context, uid int, req *message.TextMessageRequest) error {

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeText,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
		Content:    html.EscapeString(req.Content),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	if err := m.db.WithContext(ctx).Create(data).Error; err != nil {
		return err
	}

	m.afterHandle(ctx, data, map[string]string{
		"text": strutil.MtSubstr(data.Content, 0, 300),
	})

	return nil
}

// SendImage 图片文件消息
func (m *MessageService) SendImage(ctx context.Context, uid int, req *message.ImageMessageRequest) error {

	parse, err := url.Parse(req.Url)
	if err != nil {
		return err
	}

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeFile,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}

		file := &model.TalkRecordsFile{
			RecordId:     data.Id,
			UserId:       uid,
			Source:       1,
			Type:         entity.MediaFileImage,
			Drive:        entity.FileDriveMode("local"),
			OriginalName: "图片名称",
			Suffix:       strutil.FileSuffix(req.Url),
			Size:         int(req.Size),
			Path:         parse.Path,
			Url:          req.Url,
		}

		return tx.Create(file).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[图片消息]"})
	}

	return err
}

// SendVoice 语音文件消息
func (m *MessageService) SendVoice(ctx context.Context, uid int, req *message.VoiceMessageRequest) error {

	parse, err := url.Parse(req.Url)
	if err != nil {
		return err
	}

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeFile,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}

		file := &model.TalkRecordsFile{
			RecordId:     data.Id,
			UserId:       uid,
			Source:       1,
			Type:         entity.MediaFileAudio,
			Drive:        entity.FileDriveMode("local"),
			OriginalName: "语音文件",
			Suffix:       strutil.FileSuffix(req.Url),
			Size:         int(req.Size),
			Path:         parse.Path,
			Url:          req.Url,
		}

		return tx.Create(file).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[语音消息]"})
	}

	return err
}

// SendVideo 视频文件消息
func (m *MessageService) SendVideo(ctx context.Context, uid int, req *message.VideoMessageRequest) error {

	parse, err := url.Parse(req.Url)
	if err != nil {
		return err
	}

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeFile,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}

		file := &model.TalkRecordsFile{
			RecordId:     data.Id,
			UserId:       uid,
			Source:       1,
			Type:         entity.MediaFileVideo,
			Drive:        entity.FileDriveMode("local"),
			OriginalName: "语音文件",
			Suffix:       strutil.FileSuffix(req.Url),
			Size:         int(req.Size),
			Path:         parse.Path,
			Url:          req.Url,
		}

		return tx.Create(file).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[视频文件消息]"})
	}

	return err
}

// SendFile 文件消息
func (m *MessageService) SendFile(ctx context.Context, uid int, req *message.FileMessageRequest) error {

	file, err := m.splitUploadRepo.GetFile(ctx, uid, req.UploadId)
	if err != nil {
		return err
	}

	filePath := fmt.Sprintf("private/files/talks/%s/%s.%s", timeutil.DateNumber(), encrypt.Md5(strutil.Random(16)), file.FileExt)
	if err := m.fileSystem.Default.Copy(file.Path, filePath); err != nil {
		logger.Error("文件拷贝失败 err: ", err.Error())
		return err
	}

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeFile,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err = tx.Create(data).Error; err != nil {
			return err
		}

		data := &model.TalkRecordsFile{
			RecordId:     data.Id,
			UserId:       uid,
			Source:       1,
			Type:         entity.MediaFileOther,
			Drive:        file.Drive,
			OriginalName: file.OriginalName,
			Suffix:       file.FileExt,
			Size:         int(file.FileSize),
			Path:         filePath,
		}

		return tx.Create(data).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[文件消息]"})
	}

	return err
}

// SendCode 代码消息
func (m *MessageService) SendCode(ctx context.Context, uid int, req *message.CodeMessageRequest) error {

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeCode,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}

		data := &model.TalkRecordsCode{
			RecordId: data.Id,
			UserId:   uid,
			Lang:     req.Lang,
			Code:     req.Code,
		}

		return tx.Create(data).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[代码消息]"})
	}

	return err
}

// SendVote 投票消息
func (m *MessageService) SendVote(ctx context.Context, uid int, req *message.VoteMessageRequest) error {

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   entity.ChatGroupMode,
		MsgType:    entity.MsgTypeVote,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	options := make(map[string]string)
	for i, value := range req.Options {
		options[fmt.Sprintf("%c", 65+i)] = value
	}

	num := m.groupMemberRepo.CountMemberTotal(ctx, int(req.Receiver.ReceiverId))

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Create(data).Error; err != nil {
			return err
		}

		data := &model.TalkRecordsVote{
			RecordId:     data.Id,
			UserId:       uid,
			Title:        req.Title,
			AnswerMode:   int(req.Mode),
			AnswerOption: jsonutil.Encode(options),
			AnswerNum:    int(num),
			IsAnonymous:  int(req.Anonymous),
		}

		return tx.Create(data).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[投票消息]"})
	}

	return err
}

// SendEmoticon 表情消息
func (m *MessageService) SendEmoticon(ctx context.Context, uid int, req *message.EmoticonMessageRequest) error {

	emoticon := &model.EmoticonItem{}
	if err := m.db.Model(&model.EmoticonItem{}).Where("id = ? and user_id = ?", req.EmoticonId, uid).First(emoticon).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("表情信息不存在")
		}

		return err
	}

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeFile,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Create(data).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.TalkRecordsFile{
			RecordId:     data.Id,
			UserId:       uid,
			Source:       2,
			Type:         entity.GetMediaType(emoticon.FileSuffix),
			OriginalName: "图片表情",
			Suffix:       emoticon.FileSuffix,
			Size:         emoticon.FileSize,
			Path:         emoticon.Url,
			Url:          emoticon.Url,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[表情包消息]"})
	}

	return err
}

// SendForward 转发消息
func (m *MessageService) SendForward(ctx context.Context, uid int, req *message.ForwardMessageRequest) error {

	// 验证转发消息合法性
	if err := m.forward.Verify(ctx, uid, req); err != nil {
		return err
	}

	var items []*logic.ForwardRecord
	// 发送方式 1:逐条发送 2:合并发送
	if req.Mode == 1 {
		items, _ = m.forward.MultiSplitForward(ctx, uid, req)
	} else {
		items, _ = m.forward.MultiMergeForward(ctx, uid, req)
	}

	for _, item := range items {
		m.rds.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(entity.MapStrAny{
			"event": entity.EventTalk,
			"data": jsonutil.Encode(entity.MapStrAny{
				"sender_id":   uid,
				"receiver_id": item.ReceiverId,
				"talk_type":   item.TalkType,
				"record_id":   item.RecordId,
			}),
		}))
	}

	return nil
}

// SendLocation 位置消息
func (m *MessageService) SendLocation(ctx context.Context, uid int, req *message.LocationMessageRequest) error {

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		TalkType:   int(req.Receiver.TalkType),
		MsgType:    entity.MsgTypeLocation,
		UserId:     uid,
		ReceiverId: int(req.Receiver.ReceiverId),
	}

	if req.Receiver.TalkType == entity.ChatGroupMode {
		data.Sequence = m.Sequence.Get(ctx, 0, int(req.Receiver.ReceiverId))
	} else {
		data.Sequence = m.Sequence.Get(ctx, uid, int(req.Receiver.ReceiverId))
	}

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Create(data).Error; err != nil {
			return err
		}

		data := &model.TalkRecordsLocation{
			RecordId:  data.Id,
			UserId:    uid,
			Longitude: req.Longitude,
			Latitude:  req.Latitude,
		}

		return tx.Create(data).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[位置消息]"})
	}

	return err
}

// SendBusinessCard 推送用户名片消息
func (m *MessageService) SendBusinessCard(_ context.Context, uid int) error {
	return nil
}

// SendLogin 推送用户登录消息
func (m *MessageService) SendLogin(ctx context.Context, uid int, req *message.LoginMessageRequest) error {

	data := &model.TalkRecords{
		MsgId:      strutil.NewUuid(),
		Sequence:   m.Sequence.Get(ctx, 4257, uid),
		TalkType:   entity.ChatPrivateMode,
		MsgType:    entity.MsgTypeLogin,
		UserId:     4257, // 机器人ID
		ReceiverId: uid,
	}

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Create(data).Error; err != nil {
			return err
		}

		data := &model.TalkRecordsLogin{
			RecordId: data.Id,
			UserId:   uid,
			Ip:       req.Ip,
			Platform: req.Platform,
			Agent:    req.Agent,
			Address:  req.Address,
			Reason:   req.Reason,
		}

		return tx.Create(data).Error
	})

	if err == nil {
		m.afterHandle(ctx, data, map[string]string{"text": "[登录消息]"})
	}

	return err
}

// 发送消息后置处理
func (m *MessageService) afterHandle(ctx context.Context, record *model.TalkRecords, opts map[string]string) {

	if record.TalkType == entity.ChatPrivateMode {
		m.unreadStorage.Incr(ctx, entity.ChatPrivateMode, record.UserId, record.ReceiverId)

		if record.MsgType == entity.MsgTypeSystemText {
			m.unreadStorage.Incr(ctx, 1, record.ReceiverId, record.UserId)
		}
	} else if record.TalkType == entity.ChatGroupMode {

		// todo 需要加缓存
		ids := m.groupMemberRepo.GetMemberIds(ctx, record.ReceiverId)
		for _, uid := range ids {

			if uid == record.UserId {
				continue
			}

			m.unreadStorage.Incr(ctx, entity.ChatGroupMode, record.ReceiverId, uid)
		}
	}

	_ = m.messageStorage.Set(ctx, record.TalkType, record.UserId, record.ReceiverId, &cache.LastCacheMessage{
		Content:  opts["text"],
		Datetime: timeutil.DateTime(),
	})

	content := jsonutil.Encode(map[string]interface{}{
		"event": entity.EventTalk,
		"data": jsonutil.Encode(map[string]interface{}{
			"sender_id":   record.UserId,
			"receiver_id": record.ReceiverId,
			"talk_type":   record.TalkType,
			"record_id":   record.Id,
		}),
	})

	// 点对点消息采用精确投递
	if record.TalkType == entity.ChatPrivateMode {
		sids := m.sidStorage.All(ctx, 1)

		// 小于三台服务器则采用全局广播
		if len(sids) <= 3 {
			if err := m.rds.Publish(ctx, entity.ImTopicChat, content).Err(); err != nil {
				logger.Error(fmt.Sprintf("[ALL]消息推送失败 %s", err.Error()))
			}

			return
		}

		for _, sid := range m.sidStorage.All(ctx, 1) {
			for _, uid := range []int{record.UserId, record.ReceiverId} {
				if !m.clientStorage.IsCurrentServerOnline(ctx, sid, entity.ImChannelChat, strconv.Itoa(uid)) {
					continue
				}

				if err := m.rds.Publish(ctx, fmt.Sprintf(entity.ImTopicChatPrivate, sid), content).Err(); err != nil {
					logger.WithFields(entity.H{
						"sid": sid,
					}).Error(fmt.Sprintf("[Private]消息推送失败 %s", err.Error()))
				}
			}
		}
	} else {
		if err := m.rds.Publish(ctx, entity.ImTopicChat, content).Err(); err != nil {
			logger.Error(fmt.Sprintf("[ALL]消息推送失败 %s", err.Error()))
		}
	}
}
