package dao

import (
	// "encoding/json"
	"kama_chat_server/internal/model"
	"kama_chat_server/pkg/enum/contact/contact_status_enum"
	"kama_chat_server/pkg/enum/group_info/group_status_enum"
	"time"

	"gorm.io/gorm"
)

type GroupDAO interface {
	CreateGroupWithContact(group *model.GroupInfo, contact *model.UserContact) error
	GetGroupsByOwner(ownerId string) ([]model.GroupInfo, error)
	GetGroupByUUID(uuid string) (*model.GroupInfo, error)
	SaveGroup(group *model.GroupInfo) error
	GetAllGroups() ([]model.GroupInfo, error)
	LeaveGroup(group *model.GroupInfo, userId string) error
	DismissGroup(groupId string) error
	DeleteGroups(uuids []string) error
	CheckGroupExists(groupId string) (*model.GroupInfo, error)
	EnterGroup(group *model.GroupInfo, contact *model.UserContact) error
	SetGroupsStatus(uuids []string, status int8) error
	UpdateGroupWithSessions(group *model.GroupInfo) error
	RemoveGroupMembers(group *model.GroupInfo, removedUUIDs []string) error
}

type groupDAOImpl struct {
	db *gorm.DB
}

func NewGroupDAO(db *gorm.DB) GroupDAO {
	return &groupDAOImpl{db: db}
}

// CreateGroupWithContact 创建群聊并添加群主联系人 (事务)
func (dao *groupDAOImpl) CreateGroupWithContact(group *model.GroupInfo, contact *model.UserContact) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		if err := tx.Create(contact).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetGroupsByOwner 获取某人创建的群
func (dao *groupDAOImpl) GetGroupsByOwner(ownerId string) ([]model.GroupInfo, error) {
	var groupList []model.GroupInfo
	err := dao.db.Order("created_at DESC").Where("owner_id = ?", ownerId).Find(&groupList).Error
	return groupList, err
}

// GetGroupByUUID 获取群详情
func (dao *groupDAOImpl) GetGroupByUUID(uuid string) (*model.GroupInfo, error) {
	var group model.GroupInfo
	err := dao.db.First(&group, "uuid = ?", uuid).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// SaveGroup 保存群信息
func (dao *groupDAOImpl) SaveGroup(group *model.GroupInfo) error {
	return dao.db.Save(group).Error
}

// GetAllGroups 获取所有群 (管理员用，包含已软删除的，如果需要完全不过滤请加 Unscoped)
func (dao *groupDAOImpl) GetAllGroups() ([]model.GroupInfo, error) {
	var groupList []model.GroupInfo
	//service原代码使用了 Unscoped
	err := dao.db.Unscoped().Find(&groupList).Error
	return groupList, err
}

// LeaveGroup 退群 (事务：跟新群成员，删除相关会话、联系人、申请)
func (dao *groupDAOImpl) LeaveGroup(group *model.GroupInfo, userId string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		// 1. 保存群成员变更
		if err := tx.Save(group).Error; err != nil {
			return err
		}
		deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}

		// 2. 删除会话
		if err := tx.Model(&model.Session{}).Where("send_id = ? AND receive_id = ?", userId, group.Uuid).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		// 3. 删除联系人 (状态改为退群)
		if err := tx.Model(&model.UserContact{}).Where("user_id = ? AND contact_id = ?", userId, group.Uuid).Updates(map[string]interface{}{
			"deleted_at": deletedAt,
			"status":     contact_status_enum.QUIT_GROUP, // 需要引用枚举，如果这里引用不到，建议在DAO接口层传递int值
		}).Error; err != nil {
			return err
		}

		// 4. 删除申请记录
		if err := tx.Model(&model.ContactApply{}).Where("contact_id = ? AND user_id = ?", group.Uuid, userId).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		return nil
	})
}

// DismissGroup 解散群 (事务)
func (dao *groupDAOImpl) DismissGroup(groupId string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}

		// 1. 软删除群组
		if err := tx.Model(&model.GroupInfo{}).Where("uuid = ?", groupId).Updates(map[string]interface{}{
			"deleted_at": deletedAt,
			"updated_at": deletedAt.Time,
		}).Error; err != nil {
			return err
		}

		// 2. 软删除所有人接收到的该群会话
		if err := tx.Model(&model.Session{}).Where("receive_id = ?", groupId).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		// 3. 软删除联系人
		if err := tx.Model(&model.UserContact{}).Where("contact_id = ?", groupId).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		// 4. 软删除申请记录
		if err := tx.Model(&model.ContactApply{}).Where("contact_id = ?", groupId).Update("deleted_at", deletedAt).Error; err != nil {
			// 原service代码在这里忽略了 EntityNotFound，Gorm Update通常不报NotFound，除非First
			return err
		}
		return nil
	})
}

// DeleteGroups 批量删除群 (事务)
func (dao *groupDAOImpl) DeleteGroups(uuids []string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}

		// 1. 删除群
		if err := tx.Model(&model.GroupInfo{}).Where("uuid IN ?", uuids).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		// 2. 删除会话
		if err := tx.Model(&model.Session{}).Where("receive_id IN ?", uuids).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		// 3. 删除联系人
		if err := tx.Model(&model.UserContact{}).Where("contact_id IN ?", uuids).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		// 4. 删除申请
		if err := tx.Model(&model.ContactApply{}).Where("contact_id IN ?", uuids).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}
		return nil
	})
}

func (dao *groupDAOImpl) CheckGroupExists(groupId string) (*model.GroupInfo, error) {
	return dao.GetGroupByUUID(groupId)
}

// EnterGroup 进群 (事务)
func (dao *groupDAOImpl) EnterGroup(group *model.GroupInfo, contact *model.UserContact) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(group).Error; err != nil {
			return err
		}
		if err := tx.Create(contact).Error; err != nil {
			return err
		}
		return nil
	})
}

// SetGroupsStatus 设置状态 (事务)
func (dao *groupDAOImpl) SetGroupsStatus(uuids []string, status int8) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.GroupInfo{}).Where("uuid IN ?", uuids).Update("status", status).Error; err != nil {
			return err
		}

		// 如果禁用，同时删除会话
		if status == group_status_enum.DISABLE {
			deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}
			if err := tx.Model(&model.Session{}).Where("receive_id IN ?", uuids).Update("deleted_at", deletedAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateGroupWithSessions 更新群信息并同步会话 (事务)
func (dao *groupDAOImpl) UpdateGroupWithSessions(group *model.GroupInfo) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(group).Error; err != nil {
			return err
		}
		// 同步更新所有会话中的群名称和头像
		if err := tx.Model(&model.Session{}).Where("receive_id = ?", group.Uuid).Updates(map[string]interface{}{
			"receive_name": group.Name,
			"avatar":       group.Avatar,
		}).Error; err != nil {
			return err
		}
		return nil
	})
}

// RemoveGroupMembers 移除成员 (事务)
func (dao *groupDAOImpl) RemoveGroupMembers(group *model.GroupInfo, removedUUIDs []string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新群成员列表
		if err := tx.Save(group).Error; err != nil {
			return err
		}

		// 2. 级联删除相关数据
		deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}

		// 删除会话
		if err := tx.Model(&model.Session{}).Where("receive_id = ? AND send_id IN ?", group.Uuid, removedUUIDs).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		// 删除联系人
		if err := tx.Model(&model.UserContact{}).Where("contact_id = ? AND user_id IN ?", group.Uuid, removedUUIDs).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		// 删除申请
		if err := tx.Model(&model.ContactApply{}).Where("contact_id = ? AND user_id IN ?", group.Uuid, removedUUIDs).Update("deleted_at", deletedAt).Error; err != nil {
			return err
		}

		return nil
	})
}
