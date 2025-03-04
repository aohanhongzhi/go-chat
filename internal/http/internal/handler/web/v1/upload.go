package v1

import (
	"fmt"
	"time"

	"go-chat/api/pb/web/v1"
	"go-chat/config"
	"go-chat/internal/pkg/filesystem"
	"go-chat/internal/pkg/ichat"
	"go-chat/internal/pkg/strutil"
	"go-chat/internal/service"
)

type Upload struct {
	config     *config.Config
	filesystem *filesystem.Filesystem
	service    *service.SplitUploadService
}

func NewUpload(config *config.Config, filesystem *filesystem.Filesystem, service *service.SplitUploadService) *Upload {
	return &Upload{config: config, filesystem: filesystem, service: service}
}

// Avatar 头像上传上传
func (u *Upload) Avatar(ctx *ichat.Context) error {

	file, err := ctx.Context.FormFile("file")
	if err != nil {
		return ctx.InvalidParams("文件上传失败！")
	}

	stream, _ := filesystem.ReadMultipartStream(file)
	object := fmt.Sprintf("public/media/image/avatar/%s/%s", time.Now().Format("20060102"), strutil.GenImageName("png", 200, 200))

	if err := u.filesystem.Default.Write(stream, object); err != nil {
		return ctx.ErrorBusiness("文件上传失败")
	}

	return ctx.Success(web.UploadAvatarResponse{
		Avatar: u.filesystem.Default.PublicUrl(object),
	})
}

// InitiateMultipart 批量上传初始化
func (u *Upload) InitiateMultipart(ctx *ichat.Context) error {

	params := &web.UploadInitiateMultipartRequest{}
	if err := ctx.Context.ShouldBindJSON(params); err != nil {
		return ctx.InvalidParams(err)
	}

	info, err := u.service.InitiateMultipartUpload(ctx.Ctx(), &service.MultipartInitiateOpts{
		Name:   params.FileName,
		Size:   params.FileSize,
		UserId: ctx.UserId(),
	})
	if err != nil {
		return ctx.ErrorBusiness(err.Error())
	}

	return ctx.Success(&web.UploadInitiateMultipartResponse{
		UploadId:  info.UploadId,
		SplitSize: 2 << 20,
	})
}

// MultipartUpload 批量分片上传
func (u *Upload) MultipartUpload(ctx *ichat.Context) error {

	params := &web.UploadMultipartRequest{}
	if err := ctx.Context.ShouldBind(params); err != nil {
		return ctx.InvalidParams(err)
	}

	file, err := ctx.Context.FormFile("file")
	if err != nil {
		return ctx.InvalidParams("文件上传失败！")
	}

	err = u.service.MultipartUpload(ctx.Ctx(), &service.MultipartUploadOpts{
		UserId:     ctx.UserId(),
		UploadId:   params.UploadId,
		SplitIndex: int(params.SplitIndex),
		SplitNum:   int(params.SplitNum),
		File:       file,
	})
	if err != nil {
		return ctx.ErrorBusiness(err.Error())
	}

	if params.SplitIndex != params.SplitNum-1 {
		return ctx.Success(&web.UploadMultipartResponse{
			IsMerge: false,
		})
	}

	return ctx.Success(&web.UploadMultipartResponse{
		UploadId: params.UploadId,
		IsMerge:  true,
	})
}
