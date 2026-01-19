package dao

import (
	"kama_chat_server/internal/model"

	"gorm.io/gorm"
)

type UserDAO interface {
	GetUserByTelephone(telephone string) (*model.UserInfo, error)
	CreateUser(user *model.UserInfo) error
}

type userDAOImpl struct {
	db *gorm.DB
}

func NewUserDAO(db *gorm.DB) UserDAO {
	return &userDAOImpl{db: db}
}

func (dao *userDAOImpl) GetUserByTelephone(telephone string) (*model.UserInfo, error) {
	var user model.UserInfo
	err := dao.db.Where("telephone = ?", telephone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *userDAOImpl) CreateUser(user *model.UserInfo) error {
	return dao.db.Create(user).Error
}
