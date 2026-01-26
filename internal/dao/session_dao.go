package dao

import (
	"kama_chat_server/internal/model"

	"gorm.io/gorm"
)

type SessionDAO interface {
	CreateSession(session *model.Session) error
	GetSessionBySendAndReceive(sendID, receiveID string) (*model.Session, error)
	GetUserSessionList(userID string) ([]*model.Session, error)
	GetGroupSessionList(groupID string) ([]*model.Session, error)
	GetSessionByUUID(uuid string) (*model.Session, error)
	UpdateSession(session *model.Session) error
}

type sessionDAOImpl struct {
	db *gorm.DB
}

func NewSessionDAO(db *gorm.DB) SessionDAO {
	return &sessionDAOImpl{db: db}
}

func (dao *sessionDAOImpl) CreateSession(session *model.Session) error {
	return dao.db.Create(session).Error
}

func (dao *sessionDAOImpl) GetSessionBySendAndReceive(sendID, receiveID string) (*model.Session, error) {
	var session model.Session

	err := dao.db.Where("send_id = ? AND receive_id = ?", sendID, receiveID).First(&session).Error
	return &session, err
}

func (dao *sessionDAOImpl) GetUserSessionList(ownerID string) ([]*model.Session, error) {
	var sessions []*model.Session
	err := dao.db.Order("created_at DESC").
		Where("send_id = ? AND receive_id LIKE 'U%'", ownerID).
		Find(&sessions).Error
	return sessions, err
}
func (dao *sessionDAOImpl) GetGroupSessionList(ownerID string) ([]*model.Session, error) {
	var sessions []*model.Session
	err := dao.db.Order("created_at DESC").
		Where("send_id = ? AND receive_id LIKE 'G%'", ownerID).
		Find(&sessions).Error
	return sessions, err
}

func (dao *sessionDAOImpl) GetSessionByUUID(uuid string) (*model.Session, error) {
	var session *model.Session

	err := dao.db.Where("uuid = ?", uuid).First(&session).Error
	return session, err
}

func (dao *sessionDAOImpl) UpdateSession(session *model.Session) error {
	return dao.db.Save(session).Error
}
