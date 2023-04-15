// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/google/wire"
	"go-chat/config"
	"go-chat/internal/gateway/internal/consume"
	chat2 "go-chat/internal/gateway/internal/consume/chat"
	"go-chat/internal/gateway/internal/consume/example"
	"go-chat/internal/gateway/internal/event"
	"go-chat/internal/gateway/internal/event/chat"
	"go-chat/internal/gateway/internal/handler"
	"go-chat/internal/gateway/internal/process"
	"go-chat/internal/gateway/internal/router"
	"go-chat/internal/logic"
	"go-chat/internal/provider"
	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/repo"
	"go-chat/internal/repository/repo/organize"
	"go-chat/internal/service"
)

// Injectors from wire.go:

func Initialize(conf *config.Config) *AppProvider {
	client := provider.NewRedisClient(conf)
	serverStorage := cache.NewSidStorage(client)
	clientStorage := cache.NewClientStorage(client, conf, serverStorage)
	roomStorage := cache.NewRoomStorage(client)
	db := provider.NewMySQLClient(conf)
	source := repo.NewSource(db, client)
	relation := cache.NewRelation(client)
	groupMember := repo.NewGroupMember(db, relation)
	groupMemberService := service.NewGroupMemberService(source, groupMember)
	sequence := cache.NewSequence(client)
	repoSequence := repo.NewSequence(db, sequence)
	messageForwardLogic := logic.NewMessageForwardLogic(db, repoSequence)
	splitUpload := repo.NewFileSplitUpload(db)
	vote := cache.NewVote(client)
	talkRecordsVote := repo.NewTalkRecordsVote(db, vote)
	filesystem := provider.NewFilesystem(conf)
	unreadStorage := cache.NewUnreadStorage(client)
	messageStorage := cache.NewMessageStorage(client)
	messageService := service.NewMessageService(source, messageForwardLogic, groupMember, splitUpload, talkRecordsVote, filesystem, unreadStorage, messageStorage, serverStorage, clientStorage, repoSequence)
	chatHandler := chat.NewHandler(client, groupMemberService, messageService)
	chatEvent := event.NewChatEvent(client, conf, roomStorage, groupMemberService, chatHandler)
	chatChannel := handler.NewChatChannel(clientStorage, chatEvent)
	exampleEvent := event.NewExampleEvent()
	exampleChannel := handler.NewExampleChannel(clientStorage, exampleEvent)
	handlerHandler := &handler.Handler{
		Chat:    chatChannel,
		Example: exampleChannel,
		Config:  conf,
	}
	jwtTokenStorage := cache.NewTokenSessionStorage(client)
	engine := router.NewRouter(conf, handlerHandler, jwtTokenStorage)
	healthSubscribe := process.NewHealthSubscribe(conf, serverStorage)
	talkRecords := repo.NewTalkRecords(db)
	talkRecordsService := service.NewTalkRecordsService(source, vote, talkRecordsVote, groupMember, talkRecords)
	contactRemark := cache.NewContactRemark(client)
	contact := repo.NewContact(db, contactRemark, relation)
	contactService := service.NewContactService(source, contact)
	organizeOrganize := organize.NewOrganize(db)
	handler2 := chat2.NewHandler(conf, clientStorage, roomStorage, talkRecordsService, contactService, organizeOrganize)
	chatSubscribe := consume.NewChatSubscribe(handler2)
	exampleHandler := example.NewHandler()
	exampleSubscribe := consume.NewExampleSubscribe(exampleHandler)
	messageSubscribe := process.NewMessageSubscribe(conf, client, chatSubscribe, exampleSubscribe)
	subServers := &process.SubServers{
		HealthSubscribe:  healthSubscribe,
		MessageSubscribe: messageSubscribe,
	}
	server := process.NewServer(subServers)
	emailClient := provider.NewEmailClient(conf)
	providers := provider.NewProviders(emailClient)
	appProvider := &AppProvider{
		Config:    conf,
		Engine:    engine,
		Coroutine: server,
		Handler:   handlerHandler,
		Providers: providers,
	}
	return appProvider
}

// wire.go:

var providerSet = wire.NewSet(provider.NewMySQLClient, provider.NewRedisClient, provider.NewFilesystem, provider.NewEmailClient, provider.NewProviders, router.NewRouter, wire.Struct(new(process.SubServers), "*"), process.NewServer, process.NewHealthSubscribe, process.NewMessageSubscribe, repo.NewSource, repo.NewTalkRecords, repo.NewTalkRecordsVote, repo.NewGroupMember, repo.NewContact, repo.NewFileSplitUpload, repo.NewSequence, organize.NewOrganize, logic.NewMessageForwardLogic, service.NewTalkRecordsService, service.NewGroupMemberService, service.NewContactService, service.NewMessageService, wire.Struct(new(handler.Handler), "*"), wire.Struct(new(AppProvider), "*"))
