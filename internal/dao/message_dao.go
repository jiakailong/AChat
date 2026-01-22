package dao

import (
	"kama_chat_server/internal/model"

	"gorm.io/gorm"
)

type MessageDAO interface {
	GetMessageListByUserID(userOneID, UserTwoID string) ([]*model.Message, error)
	GetMessageListByGroupID(groupID string) ([]*model.Message, error)
}

type messageDAOImpl struct {
	db *gorm.DB
}

func NewMessageDAO(db *gorm.DB) MessageDAO {
	return &messageDAOImpl{db: db}
}

func (dao *messageDAOImpl) GetMessageListByUserID(userOneID, UserTwoID string) ([]*model.Message, error) {
	var messages []*model.Message

	err := dao.db.Where("(send_id = ? AND receive_id = ?) OR (send_id = ? AND receive_id = ?)",
		userOneID, UserTwoID, UserTwoID, userOneID).
		Order("created_at ASC").
		Find(&messages).Error

	return messages, err
}

func (dao *messageDAOImpl) GetMessageListByGroupID(groupID string) ([]*model.Message, error) {
	var messages []*model.Message

	err := dao.db.Where("group_id = ?", groupID).
		Order("created_at ASC").
		Find(&messages).Error

	return messages, err
}
