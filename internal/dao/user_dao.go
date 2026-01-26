package dao

import (
	"kama_chat_server/internal/model"

	"gorm.io/gorm"
)

type UserDAO interface {
	GetUserByTelephone(telephone string) (*model.UserInfo, error)
	CreateUser(user *model.UserInfo) error
	GetUserByUUID(uuid string) (*model.UserInfo, error)
	UpdateUser(user *model.UserInfo) error
	BatchUpdateUsers(uuids []string, status int8) error
	GetNormalUserList(ownerId string) ([]model.UserInfo, error)
	AbleUsersByUUIDs(uuids []string) ([]*model.UserInfo, error)
	DisableUsers(uuids []string, status int8) error
	DeleteUsers(uuids []string) error
	BatchSetAdmin(uuids []string, isAdmin int8) error

	GetUserContact(userId, contactId string) (*model.UserContact, error)
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

func (dao *userDAOImpl) GetUserByUUID(uuid string) (*model.UserInfo, error) {
	var user model.UserInfo
	err := dao.db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *userDAOImpl) UpdateUser(user *model.UserInfo) error {
	return dao.db.Save(user).Error
}

func (dao *userDAOImpl) BatchUpdateUsers(uuids []string, status int8) error {
	return dao.db.Model(&model.UserInfo{}).Where("uuid IN ?", uuids).Update("status", status).Error
}

func (dao *userDAOImpl) GetNormalUserList(ownerId string) ([]model.UserInfo, error) {
	var users []model.UserInfo
	err := dao.db.Where("uuid != ? ", ownerId).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (dao *userDAOImpl) AbleUsersByUUIDs(uuidList []string) ([]*model.UserInfo, error) {
	var users []*model.UserInfo
	err := dao.db.Model(model.UserInfo{}).Where("uuid in (?)", uuidList).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (dao *userDAOImpl) DisableUsers(uuids []string, status int8) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.UserInfo{}).Where("uuid in ?", uuids).Update("status", status).Error; err != nil {
			return err
		}

		if err := tx.Where("send_id in ? or receives_id in ?", uuids, uuids).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		return nil
	})

}

func (dao *userDAOImpl) DeleteUsers(uuids []string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("uuid in ?", uuids).Delete(&model.UserInfo{}).Error; err != nil {
			return err
		}

		if err := tx.Where("send_id in ? or receives_id in ?", uuids, uuids).Delete(&model.Session{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id in ? or contact_id in ?", uuids, uuids).Delete(&model.UserContact{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id in ? or contact_id in ?", uuids, uuids).Delete(&model.ContactApply{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (dao *userDAOImpl) BatchSetAdmin(uuids []string, isAdmin int8) error {
	return dao.db.Model(&model.UserInfo{}).Where("uuid in ?", uuids).Update("is_admin", isAdmin).Error
}

func (dao *userDAOImpl) GetUserContact(userId, contactId string) (*model.UserContact, error) {
	var contact model.UserContact
	err := dao.db.Where("user_id = ? AND contact_id = ?", userId, contactId).First(&contact).Error
	return &contact, err
}
