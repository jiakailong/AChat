package dao

import (
	"time"

	"gorm.io/gorm"

	"kama_chat_server/internal/model"
	"kama_chat_server/pkg/enum/contact/contact_status_enum"
)

type UserContactDAO interface {
	GetUserContacts(ownerId string) ([]model.UserContact, error)
	GetJoinedGroupContacts(ownerId string) ([]model.UserContact, error)
	GetContactApply(userId, contactId string) (*model.ContactApply, error)
	CreateContactApply(apply *model.ContactApply) error
	UpdateContactApply(apply *model.ContactApply) error
	GetContactApplyListByContactAndStatus(contactId string, status int8) ([]model.ContactApply, error)
	CreateUserContact(contact *model.UserContact) error
	UpdateUserContact(contact *model.UserContact) error
	GetUserContact(userId, contactId string) (*model.UserContact, error)
	DeleteContactCascade(ownerId, contactId string, deletedAt gorm.DeletedAt) error
	BlackContactCascade(ownerId, contactId string, deletedAt gorm.DeletedAt, updatedAt time.Time) error
	CancelBlackContact(ownerId, contactId string, updatedAt time.Time) error
}

type userContactDAOImpl struct {
	db *gorm.DB
}

func NewUserContactDAO(db *gorm.DB) UserContactDAO {
	return &userContactDAOImpl{db: db}
}

func (dao *userContactDAOImpl) GetUserContacts(ownerId string) ([]model.UserContact, error) {
	var contacts []model.UserContact
	err := dao.db.Order("created_at DESC").
		Where("user_id = ? AND status != ?", ownerId, contact_status_enum.DELETE).
		Find(&contacts).Error
	return contacts, err
}

func (dao *userContactDAOImpl) GetJoinedGroupContacts(ownerId string) ([]model.UserContact, error) {
	var contacts []model.UserContact
	err := dao.db.Order("created_at DESC").
		Where("user_id = ? AND status != ? AND status != ?", ownerId, contact_status_enum.QUIT_GROUP, contact_status_enum.KICK_OUT_GROUP).
		Find(&contacts).Error
	return contacts, err
}

func (dao *userContactDAOImpl) GetContactApply(userId, contactId string) (*model.ContactApply, error) {
	var apply model.ContactApply
	err := dao.db.Where("user_id = ? AND contact_id = ?", userId, contactId).First(&apply).Error
	return &apply, err
}

func (dao *userContactDAOImpl) CreateContactApply(apply *model.ContactApply) error {
	return dao.db.Create(apply).Error
}

func (dao *userContactDAOImpl) UpdateContactApply(apply *model.ContactApply) error {
	return dao.db.Save(apply).Error
}

func (dao *userContactDAOImpl) GetContactApplyListByContactAndStatus(contactId string, status int8) ([]model.ContactApply, error) {
	var applies []model.ContactApply
	err := dao.db.Where("contact_id = ? AND status = ?", contactId, status).Find(&applies).Error
	return applies, err
}

func (dao *userContactDAOImpl) CreateUserContact(contact *model.UserContact) error {
	return dao.db.Create(contact).Error
}

func (dao *userContactDAOImpl) UpdateUserContact(contact *model.UserContact) error {
	return dao.db.Save(contact).Error
}

func (dao *userContactDAOImpl) GetUserContact(userId, contactId string) (*model.UserContact, error) {
	var contact model.UserContact
	err := dao.db.Where("user_id = ? AND contact_id = ?", userId, contactId).First(&contact).Error
	return &contact, err
}

func (dao *userContactDAOImpl) DeleteContactCascade(ownerId, contactId string, deletedAt gorm.DeletedAt) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", ownerId, contactId).
			Updates(map[string]interface{}{
				"deleted_at": deletedAt,
				"status":     contact_status_enum.DELETE,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", contactId, ownerId).
			Updates(map[string]interface{}{
				"deleted_at": deletedAt,
				"status":     contact_status_enum.BE_DELETE,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Session{}).
			Where("send_id = ? AND receive_id = ?", ownerId, contactId).
			Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Session{}).
			Where("send_id = ? AND receive_id = ?", contactId, ownerId).
			Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContactApply{}).
			Where("contact_id = ? AND user_id = ?", ownerId, contactId).
			Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContactApply{}).
			Where("contact_id = ? AND user_id = ?", contactId, ownerId).
			Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		return nil
	})
}

func (dao *userContactDAOImpl) BlackContactCascade(ownerId, contactId string, deletedAt gorm.DeletedAt, updatedAt time.Time) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", ownerId, contactId).
			Updates(map[string]interface{}{
				"status":    contact_status_enum.BLACK,
				"update_at": updatedAt,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", contactId, ownerId).
			Updates(map[string]interface{}{
				"status":    contact_status_enum.BE_BLACK,
				"update_at": updatedAt,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Session{}).
			Where("send_id = ? AND receive_id = ?", ownerId, contactId).
			Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		return nil
	})
}

func (dao *userContactDAOImpl) CancelBlackContact(ownerId, contactId string, updatedAt time.Time) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", ownerId, contactId).
			Updates(map[string]interface{}{
				"status":    contact_status_enum.NORMAL,
				"update_at": updatedAt,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UserContact{}).
			Where("user_id = ? AND contact_id = ?", contactId, ownerId).
			Updates(map[string]interface{}{
				"status":    contact_status_enum.NORMAL,
				"update_at": updatedAt,
			}).Error; err != nil {
			return err
		}
		return nil
	})
}
