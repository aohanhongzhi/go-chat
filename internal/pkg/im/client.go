package im

import (
	"context"
	"fmt"
	"time"

	"github.com/tidwall/gjson"
	"go-chat/internal/pkg/jsonutil"
)

type IClient interface {
	Cid() int64                         // 客户端ID
	Uid() int                           // 客户端关联用户ID
	Close(code int, text string)        // 关闭客户端
	Write(data *ClientOutContent) error // 写入数据
}

type IStorage interface {
	Bind(ctx context.Context, channel string, cid int64, uid int)
	UnBind(ctx context.Context, channel string, cid int64)
}

// ClientInContent 客户端接收消息体
type ClientInContent struct {
	IsAck   bool   // 是否需要 ack 回调
	Event   string // 消息事件
	Content []byte // 消息内容
}

// ClientOutContent 客户端输出的消息体
type ClientOutContent struct {
	AckId   string // ACK ID（唯一性）
	IsAck   bool   // 是否需要 ack 回调
	Retry   int    // 重试次数
	Content []byte // 消息内容
}

// Client WebSocket 客户端连接信息
type Client struct {
	conn     IConn                  // 客户端连接
	cid      int64                  // 客户端ID/客户端唯一标识
	uid      int                    // 用户ID
	lastTime int64                  // 客户端最后心跳时间/心跳检测
	channel  *Channel               // 渠道分组
	isClosed bool                   // 客户端是否关闭连接
	outChan  chan *ClientOutContent // 发送通道
	storage  IStorage               // 缓存服务
	callBack ICallback              // 回调方法
}

type ClientOption struct {
	Uid     int      // 用户识别ID
	Channel *Channel // 渠道信息
	Storage IStorage // 自定义缓存组件，用于绑定用户与客户端的关系
	Buffer  int      // 缓冲区大小根据业务，自行调整
}

// NewClient 初始化客户端信息
func NewClient(ctx context.Context, conn IConn, opt *ClientOption, callBack ICallback) IClient {

	if opt.Buffer <= 0 {
		opt.Buffer = 10
	}

	if callBack == nil {
		panic("callBack nil")
	}

	client := &Client{
		conn:     conn,
		cid:      Counter.GenID(),
		lastTime: time.Now().Unix(),
		uid:      opt.Uid,
		channel:  opt.Channel,
		storage:  opt.Storage,
		outChan:  make(chan *ClientOutContent, opt.Buffer),
		callBack: callBack,
	}

	// 设置客户端连接关闭回调事件
	conn.SetCloseHandler(client.close)

	// 绑定客户端映射关系
	if client.storage != nil {
		client.storage.Bind(ctx, client.channel.name, client.cid, client.uid)
	}

	// 注册客户端
	client.channel.addClient(client)

	// 触发自定义的 Open 事件
	client.callBack.Open(client)

	// 注册心跳管理
	health.addClient(client)

	return client.initialize()
}

// Cid 获取客户端ID
func (c *Client) Cid() int64 {
	return c.cid
}

// Uid 获取客户端关联的用户ID
func (c *Client) Uid() int {
	return c.uid
}

// Close 关闭客户端连接
func (c *Client) Close(code int, message string) {
	defer func() {
		_ = c.conn.Close()
	}()

	// 触发客户端关闭回调事件
	_ = c.close(code, message)
}

// Write 客户端写入数据
func (c *Client) Write(data *ClientOutContent) error {

	if c.isClosed {
		return fmt.Errorf("connection closed")
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("client write err :%v \n", err)
		}
	}()

	// 消息写入缓冲通道
	c.outChan <- data

	return nil
}

// 推送心跳检测配置
func (c *Client) heartbeat() {
	_ = c.Write(&ClientOutContent{
		Content: jsonutil.Marshal(&Message{
			Event: "connect",
			Content: map[string]interface{}{
				"ping_interval": heartbeatInterval,
				"ping_timeout":  heartbeatTimeout,
			},
		}),
	})
}

// 关闭回调
func (c *Client) close(code int, text string) error {

	if !c.isClosed {
		c.isClosed = true
		close(c.outChan) // 关闭通道

		// 触发连接关闭回调
		c.callBack.Close(c, code, text)
	}

	// 解绑关联
	if c.storage != nil {
		c.storage.UnBind(context.Background(), c.channel.name, c.cid)
	}

	// 渠道分组移除客户端
	c.channel.delClient(c)

	// 心跳管理移除客户端
	health.delClient(c)

	return nil
}

// 循环接收客户端推送信息
func (c *Client) loopAccept() {
	defer func() {
		_ = c.conn.Close()
	}()

	for {
		// 读取客户端中的数据
		message, err := c.conn.Read()
		if err != nil {
			return
		}

		// 更新最后心跳时间
		c.lastTime = time.Now().Unix()

		result := gjson.GetBytes(message, "event")
		if !result.Exists() {
			continue
		}

		switch result.String() {
		case "heartbeat": // 心跳消息判断
			_ = c.Write(&ClientOutContent{
				Content: jsonutil.Marshal(&Message{"heartbeat", "pong"}),
			})
		default:
			// 触发消息回调
			c.callBack.Message(c, message)
		}
	}
}

// 循环推送客户端信息
func (c *Client) loopWrite() {
	for data := range c.outChan {

		if c.isClosed {
			break
		}

		if err := c.conn.Write(data.Content); err != nil {
			break
		}

		// 验证是否需要 ack 回调
		if data.IsAck {
			ack.add(&AckBufferOption{
				Channel: c.channel,
				Cid:     c.Cid(),
				AckID:   data.AckId,
				Retry:   data.Retry + 1,
				Content: data.Content,
			})
		}
	}
}

// 初始化连接
func (c *Client) initialize() *Client {
	// 推送心跳检测配置
	c.heartbeat()

	// 启动协程处理接收信息
	go c.loopAccept()

	// 启动协程处理推送信息
	go c.loopWrite()

	return c
}
